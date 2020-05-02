package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/integrations/gitlab/api"
	"github.com/pinpt/agent/integrations/pkg/commiturl"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/agent/pkg/ids2"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type Config struct {
	commonrepo.FilterConfig
	URL                string `json:"url"`
	APIKey             string `json:"api_key"`
	AccessToken        string `json:"access_token"`
	OnlyGit            bool   `json:"only_git"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc api.QueryContext

	config Config

	requestConcurrencyChan chan bool

	refType string

	isGitlabCom bool
}

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}

func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}
func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	s.refType = "gitlab"

	s.qc = api.QueryContext{
		Logger: s.logger,
	}

	return nil
}

func (s *Integration) ValidateConfig(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {

	rerr := func(err error) {
		res.Errors = append(res.Errors, err.Error())
	}

	err := s.initWithConfig(exportConfig)
	if err != nil {
		rerr(err)
		return
	}

	res.ServerVersion, err = api.ServerVersion(s.qc)
	if err != nil {
		rerr(err)
		return
	}

	groups, err := api.GroupsAll(s.qc)
	if err != nil {
		rerr(err)
		return
	}

	params := url.Values{}
	params.Set("per_page", "1")

LOOP:
	for _, group := range groups {
		_, repos, err := api.ReposPageCommon(s.qc, group, params)
		if err != nil {
			rerr(err)
			return
		}
		if len(repos) > 0 {
			repoURL, err := s.getRepoURL(repos[0].NameWithOwner)
			if err != nil {
				rerr(err)
				return
			}
			res.RepoURL = repoURL
			break LOOP
		}
	}

	return
}

func (s *Integration) Export(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ExportResult, rerr error) {
	err := s.initWithConfig(exportConfig)
	if err != nil {
		rerr = err
		return
	}

	projects, err := s.export(ctx)
	if err != nil {
		rerr = err
		return
	}

	res.Projects = projects
	return
}

func (s *Integration) initWithConfig(config rpcdef.ExportConfig) error {
	err := s.setIntegrationConfig(config.Integration)
	if err != nil {
		return err
	}

	s.qc.BaseURL = s.config.URL
	s.qc.CustomerID = config.Pinpoint.CustomerID
	s.qc.Logger = s.logger
	s.qc.RefType = s.refType
	s.customerID = config.Pinpoint.CustomerID
	{
		opts := api.RequesterOpts{}
		opts.Logger = s.logger
		opts.APIURL = s.config.URL + "/api/v4"
		opts.APIKey = s.config.APIKey
		opts.AccessToken = s.config.AccessToken
		opts.InsecureSkipVerify = s.config.InsecureSkipVerify
		opts.Concurrency = make(chan bool, 10)
		opts.Agent = s.agent
		requester := api.NewRequester(opts)

		s.qc.Request = requester.MakeRequest
		s.qc.IDs = ids2.New(s.customerID, s.refType)
	}

	return nil
}

func (s *Integration) setIntegrationConfig(data rpcdef.IntegrationConfig) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}
	var conf Config
	if err := structmarshal.MapToStruct(data.Config, &conf); err != nil {
		return err
	}
	if conf.AccessToken == "" {
		if conf.APIKey == "" {
			return rerr("api_key are missing")
		}
		if conf.URL == "" {
			return rerr("url missing")
		}
	} else {
		conf.URL = "https://gitlab.com"
	}
	if conf.URL == "https://www.gitlab.com" {
		conf.URL = "https://gitlab.com"
	}
	u, err := url.Parse(conf.URL)
	if err != nil {
		return rerr(fmt.Sprintf("url is not valid: %v", err))
	}
	s.isGitlabCom = u.Hostname() == "gitlab.com"
	s.config = conf
	return nil
}

func (s *Integration) export(ctx context.Context) (repoResults []rpcdef.ExportProject, rerr error) {

	if !s.isGitlabCom {
		if err := UsersEmails(s); err != nil {
			rerr = err
			return
		}
	}

	groupNames, err := api.GroupsAll(s.qc)
	if err != nil {
		rerr = err
		return
	}

	groupSession, err := objsender.RootTracking(s.agent, "group")
	if err != nil {
		rerr = err
		return
	}
	if err = groupSession.SetTotal(len(groupNames)); err != nil {
		rerr = err
		return
	}

	for _, groupName := range groupNames {
		groupResults, err := s.exportGroup(ctx, groupSession, groupName)
		if err != nil {
			rerr = err
			return
		}
		if err := groupSession.IncProgress(); err != nil {
			rerr = err
			return
		}
		repoResults = append(repoResults, groupResults...)
	}

	err = groupSession.Done()
	if err != nil {
		rerr = err
		return
	}

	return
}

func (s *Integration) exportGit(repo commonrepo.Repo, prs []rpcdef.GitRepoFetchPR) error {
	repoURL, err := s.getRepoURL(repo.NameWithOwner)
	if err != nil {
		return err
	}

	args := rpcdef.GitRepoFetch{}
	args.RepoID = s.qc.IDs.CodeRepo(repo.ID)
	args.UniqueName = repo.NameWithOwner
	args.RefType = s.refType
	args.URL = repoURL
	args.CommitURLTemplate = commiturl.CommitURLTemplate(repo, s.config.URL)
	args.BranchURLTemplate = commiturl.BranchURLTemplate(repo, s.config.URL)
	args.PRs = prs
	if err = s.agent.ExportGitRepo(args); err != nil {
		return err
	}
	return nil
}

func (s *Integration) exportGroup(ctx context.Context, groupSession *objsender.Session, groupName string) (_ []rpcdef.ExportProject, rerr error) {

	s.logger.Info("exporting group", "name", groupName)
	logger := s.logger.With("org", groupName)

	repos, err := commonrepo.ReposAllSlice(func(res chan []commonrepo.Repo) error {
		return api.ReposAll(s.qc, groupName, res)
	})
	if err != nil {
		rerr = err
		return
	}

	repos = commonrepo.Filter(logger, repos, s.config.FilterConfig)

	if s.config.OnlyGit {
		logger.Warn("only_ripsrc flag passed, skipping export of data from gitlab api")
		for _, repo := range repos {
			err := s.exportGit(repo, nil)
			if err != nil {
				rerr = err
				return
			}
		}
		return
	}

	repoSender, err := groupSession.Session(sourcecode.RepoModelName.String(), groupName, groupName)
	if err != nil {
		rerr = err
		return
	}

	repoSender.SetNoAutoProgress(true)

	if err = repoSender.SetTotal(len(repos)); err != nil {
		rerr = err
		return
	}

	if err = s.exportRepos(ctx, logger, repoSender, groupName, repos); err != nil {
		rerr = err
		return
	}

	var reposIface []repoprojects.RepoProject
	for _, r := range repos {
		reposIface = append(reposIface, r)
	}

	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.logger
	processOpts.ProjectFn = func(ctx *repoprojects.ProjectCtx) error {
		repo := ctx.Project.(commonrepo.Repo)
		return s.exportRepoChildren(ctx, repo)
	}
	processOpts.Concurrency = 1
	processOpts.Projects = reposIface

	processOpts.IntegrationType = inconfig.IntegrationTypeSourcecode
	processOpts.CustomerID = s.customerID
	processOpts.RefType = s.refType
	processOpts.Sender = repoSender

	processor := repoprojects.NewProcess(processOpts)
	exportResult, err := processor.Run()
	if err != nil {
		rerr = err
		return
	}

	err = repoSender.Done()
	if err != nil {
		rerr = err
		return
	}

	return exportResult, nil
}

func (s *Integration) exportRepoChildren(ctx *repoprojects.ProjectCtx, repo commonrepo.Repo) error {

	if s.isGitlabCom {
		err := s.exportUsersFromRepo(ctx, repo)
		if err != nil {
			return err
		}
	}

	prs, err := s.exportPullRequestsForRepo(ctx, repo)
	if err != nil {
		return err
	}

	return s.exportGit(repo, prs)
}

func (s *Integration) exportRepos(ctx context.Context, logger hclog.Logger, sender *objsender.Session, groupName string, onlyInclude []commonrepo.Repo) error {

	shouldInclude := map[string]bool{}
	for _, repo := range onlyInclude {
		shouldInclude[repo.NameWithOwner] = true
	}

	err := api.PaginateNewerThan(s.logger, sender.LastProcessedTime(), func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, repos, err := api.ReposPage(s.qc, groupName, parameters, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		for _, repo := range repos {
			if !shouldInclude[repo.Name] {
				continue
			}
			err := sender.Send(repo)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})

	if err != nil {
		return err
	}

	return nil
}

// used for gitlab.com
func (s *Integration) exportUsersFromRepo(ctx *repoprojects.ProjectCtx, repo commonrepo.Repo) error {
	sender, err := ctx.Session(sourcecode.UserModelName)
	if err != nil {
		return err
	}

	return api.PaginateStartAt(s.logger, func(log hclog.Logger, parameters url.Values) (api.PageInfo, error) {
		pi, users, err := api.RepoUsersPageREST(s.qc, repo, parameters)
		if err != nil {
			return pi, err
		}
		for _, user := range users {
			err := sender.Send(user)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})
}

func (s *Integration) exportPullRequestsForRepo(ctx *repoprojects.ProjectCtx, repo commonrepo.Repo) (res []rpcdef.GitRepoFetchPR, rerr error) {

	pullRequestSender, err := ctx.Session(sourcecode.PullRequestModelName)
	if err != nil {
		rerr = err
		return
	}

	commitsSender, err := ctx.Session(sourcecode.PullRequestCommitModelName)
	if err != nil {
		rerr = err
		return
	}

	logger := ctx.Logger.With("repo", repo.NameWithOwner)
	logger.Info("exporting")

	// export changed pull requests
	pullRequestsInitial := make(chan []api.PullRequest)
	// export comments, reviews, commits concurrently
	pullRequestsForComments := make(chan []api.PullRequest, 10)
	pullRequestsForReviews := make(chan []api.PullRequest, 10)
	pullRequestsForCommits := make(chan []api.PullRequest, 10)

	var errMu sync.Mutex
	setErr := func(err error, chanToDrain chan []api.PullRequest) {
		logger.Error("failed repo export", "e", err)

		errMu.Lock()
		defer errMu.Unlock()
		// comment this out for now, PROD-460
		// if rerr == nil {
		// 	rerr = err
		// }

		// drain all pull requests on error
		for range chanToDrain {
		}
	}

	go func() {
		defer close(pullRequestsInitial)
		if err := s.exportPullRequestsRepo(logger, repo, pullRequestSender, pullRequestsInitial, pullRequestSender.LastProcessedTime()); err != nil {
			setErr(err, pullRequestsInitial)
		}
	}()

	go func() {
		for item := range pullRequestsInitial {
			pullRequestsForComments <- item
			pullRequestsForReviews <- item
			pullRequestsForCommits <- item
		}
		close(pullRequestsForComments)
		close(pullRequestsForReviews)
		close(pullRequestsForCommits)
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.exportPullRequestsComments(logger, pullRequestSender, repo, pullRequestsForComments); err != nil {
			setErr(fmt.Errorf("error getting comments %s", err), pullRequestsForComments)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.exportPullRequestsReviews(logger, pullRequestSender, repo, pullRequestsForReviews); err != nil {
			setErr(fmt.Errorf("error getting reviews %s", err), pullRequestsForReviews)
		}
	}()

	// set commits on the rp and then send the pr
	wg.Add(1)
	go func() {
		defer wg.Done()
		for prs := range pullRequestsForCommits {
			for _, pr := range prs {
				commits, err := s.exportPullRequestCommits(logger, repo, pr.RefID, pr.IID)
				if err != nil {
					setErr(fmt.Errorf("error getting commits %s", err), pullRequestsForCommits)
					return
				}

				commitsSender.SetTotal(len(commits))

				if len(commits) > 0 {
					meta := rpcdef.GitRepoFetchPR{}
					repoID := s.qc.IDs.CodeRepo(repo.ID)
					meta.ID = s.qc.IDs.CodePullRequest(repoID, pr.RefID)
					meta.RefID = pr.RefID
					meta.URL = pr.URL
					meta.BranchName = pr.BranchName
					meta.LastCommitSHA = commits[0].Sha
					res = append(res, meta)
				}
				for ind := len(commits) - 1; ind >= 0; ind-- {
					pr.CommitShas = append(pr.CommitShas, commits[ind].Sha)
				}

				pr.CommitIds = ids.CodeCommits(s.qc.CustomerID, s.refType, pr.RepoID, pr.CommitShas)
				if len(pr.CommitShas) == 0 {
					logger.Info("found PullRequest with no commits (ignoring it)", "repo", repo.NameWithOwner, "pr_ref_id", pr.RefID, "pr.url", pr.URL)
				} else {
					pr.BranchID = s.qc.IDs.CodeBranch(pr.RepoID, pr.BranchName, pr.CommitShas[0])
				}
				if err = pullRequestSender.Send(pr); err != nil {
					setErr(err, pullRequestsForCommits)
					return
				}

				for _, c := range commits {
					c.BranchID = pr.BranchID
					if err := commitsSender.Send(c); err != nil {
						setErr(err, pullRequestsForCommits)
						return
					}
				}
			}
		}
	}()
	wg.Wait()
	return
}

func (s *Integration) getRepoURL(nameWithOwner string) (string, error) {
	u, err := url.Parse(s.config.URL)
	if err != nil {
		return "", err
	}
	if s.config.AccessToken != "" {
		u.User = url.UserPassword("oauth2", s.config.AccessToken)
	} else if s.config.APIKey != "" {
		u.User = url.UserPassword("token", s.config.APIKey)
	} else {
		return "", errors.New("no APIKey or AccessToken passed to getRepoURL")
	}
	u.Path = nameWithOwner
	return u.String(), nil
}
