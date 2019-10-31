package main

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
	"github.com/pinpt/agent.next/integrations/pkg/commiturl"
	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/integrations/pkg/objsender"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/pkg/ids2"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/sourcecode"
)

const (
	CLOUD      = "cloud"
	ON_PREMISE = "on-premise"
)

type Config struct {
	commonrepo.FilterConfig
	URL                string `json:"url"`
	APIToken           string `json:"apitoken"`
	OnlyGit            bool   `json:"only_git"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	ServerType         string `json:"server_type"`
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc api.QueryContext

	config Config

	requestConcurrencyChan chan bool

	refType string
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

	groups, err := api.Groups(s.qc)
	if err != nil {
		rerr(err)
		return
	}

	params := url.Values{}
	params.Set("per_page", "1")

	for _, group := range groups {
		_, repos, err := api.ReposPageRESTAll(s.qc, group, params)
		if err != nil {
			rerr(err)
			return
		}
		if len(repos) > 0 {
			repoURL, err := getRepoURL(s.config.URL, url.UserPassword("token", s.config.APIToken), repos[0].NameWithOwner)
			if err != nil {
				rerr(err)
				return
			}
			res.ReposURLs = append(res.ReposURLs, repoURL)
		}
	}

	return
}

func (s *Integration) Export(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ExportResult, err error) {
	err = s.initWithConfig(exportConfig)
	if err != nil {
		return
	}

	err = s.export(ctx)

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
		opts.APIGraphQL = s.config.URL + "/api/graphql"
		opts.APIToken = s.config.APIToken
		opts.InsecureSkipVerify = s.config.InsecureSkipVerify
		requester := api.NewRequester(opts)

		s.qc.Request = requester.Request
		s.qc.RequestGraphQL = requester.RequestGraphQL
		s.qc.IDs = ids2.New(s.customerID, s.refType)
	}

	return nil
}

func (s *Integration) setIntegrationConfig(data map[string]interface{}) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}
	var conf Config
	err := structmarshal.MapToStruct(data, &conf)
	if err != nil {
		return err
	}
	if conf.URL == "" {
		return rerr("url is missing")
	}
	if conf.APIToken == "" {
		return rerr("api token is missing")
	}
	if conf.ServerType == "" {
		return rerr("server type is missing")
	}
	if conf.ServerType != CLOUD && conf.ServerType != ON_PREMISE {
		return rerr("server type invalid: type %s", conf.ServerType)
	}

	s.config = conf
	return nil
}

func (s *Integration) export(ctx context.Context) (err error) {

	if s.config.ServerType == ON_PREMISE {
		if err = UsersEmails(s); err != nil {
			return err
		}
	}

	groupNames, err := api.Groups(s.qc)
	if err != nil {
		return err
	}

	groupSession, err := objsender.RootTracking(s.agent, "group")
	if err != nil {
		return err
	}
	if err = groupSession.SetTotal(len(groupNames)); err != nil {
		return err
	}

	for _, groupName := range groupNames {
		if err := s.exportGroup(ctx, groupSession, groupName); err != nil {
			return err
		}
		if err := groupSession.IncProgress(); err != nil {
			return err
		}
	}

	return groupSession.Done()
}

func (s *Integration) exportGit(repo commonrepo.Repo, prs []rpcdef.GitRepoFetchPR) error {
	repoURL, err := getRepoURL(s.config.URL, url.UserPassword("token", s.config.APIToken), repo.NameWithOwner)
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
	for _, pr := range prs {
		pr2 := rpcdef.GitRepoFetchPR{}
		pr2.ID = pr.ID
		pr2.RefID = pr.RefID
		pr2.LastCommitSHA = pr.LastCommitSHA
		args.PRs = append(args.PRs, pr2)
	}
	if err = s.agent.ExportGitRepo(args); err != nil {
		return err
	}
	return nil
}

func (s *Integration) exportGroup(ctx context.Context, groupSession *objsender.Session, groupName string) error {
	s.logger.Info("exporting group", "name", groupName)
	logger := s.logger.With("org", groupName)

	repos, err := commonrepo.ReposAllSlice(func(res chan []commonrepo.Repo) error {
		return api.ReposAll(s.qc, groupName, res)
	})
	if err != nil {
		return err
	}

	repos = commonrepo.Filter(logger, repos, s.config.FilterConfig)

	if s.config.OnlyGit {
		logger.Warn("only_ripsrc flag passed, skipping export of data from gitlab api")
		for _, repo := range repos {
			err := s.exportGit(repo, nil)
			if err != nil {
				return err
			}
		}
		return nil
	}

	repoSender, err := groupSession.Session(sourcecode.RepoModelName.String(), groupName, groupName)
	if err != nil {
		return err
	}

	// export repos
	if err = s.exportRepos(ctx, logger, repoSender, groupName, repos); err != nil {
		return err
	}
	if err = repoSender.SetTotal(len(repos)); err != nil {
		return err
	}

	if s.config.ServerType == CLOUD {
		userSender, err := objsender.Root(s.agent, sourcecode.UserModelName.String())
		if err != nil {
			return err
		}
		for _, repo := range repos {
			err = s.exportUsersFromRepos(ctx, logger, userSender, repo)
			if err != nil {
				return err
			}
		}

		if err := userSender.Done(); err != nil {
			return err
		}
	}

	for _, repo := range repos {

		prSender, err := repoSender.Session(sourcecode.PullRequestModelName.String(), repo.ID, repo.NameWithOwner)
		if err != nil {
			return err
		}

		prCommitsSender, err := repoSender.Session(sourcecode.PullRequestCommitModelName.String(), repo.ID, repo.NameWithOwner)
		if err != nil {
			return err
		}

		prs, err := s.exportPullRequestsForRepo(logger, repo, prSender, prCommitsSender)
		if err != nil {
			return err
		}

		if err = s.exportGit(repo, prs); err != nil {
			return err
		}

		if err = prSender.Done(); err != nil {
			return err
		}

		if err = prCommitsSender.Done(); err != nil {
			return err
		}
	}

	return repoSender.Done()
}

func (s *Integration) exportRepos(ctx context.Context, logger hclog.Logger, sender *objsender.Session, groupName string, onlyInclude []commonrepo.Repo) error {

	shouldInclude := map[string]bool{}
	for _, repo := range onlyInclude {
		shouldInclude[repo.NameWithOwner] = true
	}

	err := api.PaginateNewerThan(s.logger, sender.LastProcessedTime(), func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, repos, err := api.ReposPageREST(s.qc, groupName, parameters, stopOnUpdatedAt)
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

func (s *Integration) exportUsersFromRepos(ctx context.Context, logger hclog.Logger, sender *objsender.Session, repo commonrepo.Repo) error {
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

func (s *Integration) exportPullRequestsForRepo(logger hclog.Logger, repo commonrepo.Repo,
	pullRequestSender *objsender.Session,
	commitsSender *objsender.Session) (res []rpcdef.GitRepoFetchPR, rerr error) {

	logger = logger.With("repo", repo.NameWithOwner)
	logger.Info("exporting")

	// export changed pull requests
	var pullRequestsErr error
	pullRequestsInitial := make(chan []api.PullRequest)
	go func() {
		defer close(pullRequestsInitial)
		if err := s.exportPullRequestsRepo(logger, repo, pullRequestSender, pullRequestsInitial, pullRequestSender.LastProcessedTime()); err != nil {
			pullRequestsErr = err
		}
	}()

	// export comments, reviews, commits concurrently
	pullRequestsForComments := make(chan []api.PullRequest, 10)
	pullRequestsForReviews := make(chan []api.PullRequest, 10)
	pullRequestsForCommits := make(chan []api.PullRequest, 10)

	var errMu sync.Mutex
	setErr := func(err error) {
		logger.Error("failed repo export", "e", err)
		errMu.Lock()
		defer errMu.Unlock()
		if rerr == nil {
			rerr = err
		}
		// drain all pull requests on error
		for range pullRequestsForComments {
		}
		for range pullRequestsForReviews {
		}
		for range pullRequestsForCommits {
		}
	}

	go func() {
		for item := range pullRequestsInitial {
			pullRequestsForComments <- item
			pullRequestsForReviews <- item
			pullRequestsForCommits <- item
		}
		close(pullRequestsForComments)
		close(pullRequestsForReviews)
		close(pullRequestsForCommits)

		if pullRequestsErr != nil {
			setErr(pullRequestsErr)
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.exportPullRequestsComments(logger, pullRequestSender, repo, pullRequestsForComments); err != nil {
			setErr(fmt.Errorf("error getting comments %s", err))
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.exportPullRequestsReviews(logger, pullRequestSender, repo, pullRequestsForReviews); err != nil {
			setErr(fmt.Errorf("error getting reviews %s", err))
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
					setErr(fmt.Errorf("error getting commits %s", err))
					return
				}

				commitsSender.SetTotal(len(commits))

				if len(commits) > 0 {
					meta := rpcdef.GitRepoFetchPR{}
					repoID := s.qc.IDs.CodeRepo(repo.ID)
					meta.ID = s.qc.IDs.CodePullRequest(repoID, pr.RefID)
					meta.RefID = pr.RefID
					meta.LastCommitSHA = commits[0].Sha
					res = append(res, meta)
				}
				for _, c := range commits {
					pr.CommitShas = append(pr.CommitShas, c.Sha)
				}

				pr.CommitIds = ids.CodeCommits(s.qc.CustomerID, s.refType, pr.RepoID, pr.CommitShas)
				if len(pr.CommitShas) == 0 {
					logger.Info("found PullRequest with no commits (ignoring it)", "repo", repo.NameWithOwner, "pr_ref_id", pr.RefID, "pr.url", pr.URL)
				} else {
					pr.BranchID = s.qc.IDs.CodeBranch(pr.RepoID, pr.BranchName, pr.CommitShas[0])
				}
				if err = pullRequestSender.Send(pr); err != nil {
					setErr(err)
					return
				}

				for _, c := range commits {
					c.BranchID = pr.BranchID
					if err := commitsSender.Send(c); err != nil {
						setErr(err)
						return
					}
				}
			}
		}
	}()
	wg.Wait()
	return
}

func getRepoURL(repoURLPrefix string, user *url.Userinfo, nameWithOwner string) (string, error) {
	u, err := url.Parse(repoURLPrefix)
	if err != nil {
		return "", err
	}
	u.User = user
	u.Path = nameWithOwner
	return u.String(), nil
}
