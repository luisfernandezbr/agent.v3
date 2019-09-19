package main

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/pkg/commitusers"
	"github.com/pinpt/agent.next/pkg/commonrepo"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/agent.next/pkg/template"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type Config struct {
	URL                string   `json:"url"`
	APIToken           string   `json:"apitoken"`
	ExcludedRepos      []string `json:"excluded_repos"`
	Repos              []string `json:"repos"`
	StopAfterN         int      `json:"stop_after_n"`
	OnlyGit            bool     `json:"only_git"`
	InsecureSkipVerify bool     `json:"insecure_skip_verify"`
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc api.QueryContext

	config Config

	requestConcurrencyChan chan bool

	refType string

	repoSender                *objsender.IncrementalDateBased
	commitUserSender          *objsender.IncrementalDateBased
	pullRequestSender         *objsender.IncrementalDateBased
	pullRequestCommentsSender *objsender.NotIncremental
	pullRequestReviewsSender  *objsender.NotIncremental
	userSender                *objsender.NotIncremental

	commonInfo commonrepo.Config
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

	if err := api.ValidateUser(s.qc); err != nil {
		rerr(err)
		return
	}

	// TODO: return a repo and validate repo that repo can be cloned in agent

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
		s.qc.BasicInfo = ids.BasicInfo{
			CustomerID: s.customerID,
			RefType:    s.refType,
		}
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
	s.config = conf
	return nil
}

func (s *Integration) export(ctx context.Context) (err error) {

	s.repoSender, err = objsender.NewIncrementalDateBased(s.agent, sourcecode.RepoModelName.String())
	if err != nil {
		return err
	}
	s.commitUserSender, err = objsender.NewIncrementalDateBased(s.agent, commitusers.TableName)
	if err != nil {
		return err
	}
	s.pullRequestSender, err = objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestModelName.String())
	if err != nil {
		return err
	}
	s.pullRequestCommentsSender = objsender.NewNotIncremental(s.agent, sourcecode.PullRequestCommentModelName.String())
	s.pullRequestReviewsSender = objsender.NewNotIncremental(s.agent, sourcecode.PullRequestReviewModelName.String())
	s.userSender = objsender.NewNotIncremental(s.agent, sourcecode.UserModelName.String())

	err = api.UsersEmails(s.qc, s.commitUserSender, s.userSender)
	if err != nil {
		return err
	}

	groupNames, err := api.Groups(s.qc)
	if err != nil {
		return err
	}

	for _, groupName := range groupNames {
		if err := s.exportGroup(ctx, groupName); err != nil {
			return err
		}
	}

	err = s.repoSender.Done()
	if err != nil {
		return err
	}
	err = s.commitUserSender.Done()
	if err != nil {
		return err
	}
	err = s.pullRequestSender.Done()
	if err != nil {
		return err
	}
	err = s.pullRequestCommentsSender.Done()
	if err != nil {
		return err
	}
	err = s.pullRequestReviewsSender.Done()
	if err != nil {
		return err
	}
	err = s.userSender.Done()
	if err != nil {
		return err
	}

	return
}

func (s *Integration) exportGroup(ctx context.Context, groupName string) error {
	s.logger.Info("exporting group", "name", groupName)
	logger := s.logger.With("org", groupName)

	repos, err := commonrepo.ReposAllSlice(s.qc, groupName, func(res chan []commonrepo.Repo) error {
		return api.ReposAll(s.qc, groupName, res)
	})
	if err != nil {
		return err
	}

	if s.config.StopAfterN != 0 {
		// only leave 1 repo for export
		stopAfter := s.config.StopAfterN
		l := len(repos)
		if len(repos) > stopAfter {
			repos = repos[0:stopAfter]
		}
		logger.Info("stop_after_n passed", "v", stopAfter, "repos", l, "after", len(repos))
	}

	// queue repos for processing with ripsrc
	{

		for _, repo := range repos {
			u, err := url.Parse(s.config.URL)
			if err != nil {
				return err
			}
			u.User = url.UserPassword("token", s.config.APIToken)
			u.Path = repo.NameWithOwner
			repoURL := u.String()

			args := rpcdef.GitRepoFetch{}
			args.RefType = s.refType
			args.RepoID = s.qc.BasicInfo.RepoID(repo.ID)
			args.URL = repoURL
			args.CommitURLTemplate = template.CommitURLTemplate(repo, s.config.URL)
			args.BranchURLTemplate = template.BranchURLTemplate(repo, s.config.URL)
			err = s.agent.ExportGitRepo(args)
			if err != nil {
				return err
			}
		}
	}

	if s.config.OnlyGit {
		logger.Warn("only_ripsrc flag passed, skipping export of data from github api")
		return nil
	}

	// export repos
	err = s.exportRepos(ctx, logger, groupName, repos)
	if err != nil {
		return err
	}

	for _, repo := range repos {
		err := s.exportPullRequestsForRepo(logger, repo, s.pullRequestSender, s.pullRequestCommentsSender, s.pullRequestReviewsSender)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Integration) exportRepos(ctx context.Context, logger hclog.Logger, groupName string, onlyInclude []commonrepo.Repo) error {

	sender := s.repoSender

	shouldInclude := map[string]bool{}
	for _, repo := range onlyInclude {
		shouldInclude[repo.NameWithOwner] = true
	}

	err := api.PaginateNewerThan(s.logger, sender.LastProcessed, func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
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

func (s *Integration) exportPullRequestsForRepo(logger hclog.Logger, repo commonrepo.Repo,
	pullRequestSender *objsender.IncrementalDateBased,
	commentsSender *objsender.NotIncremental,
	reviewSender *objsender.NotIncremental) (rerr error) {

	logger = logger.With("repo", repo.NameWithOwner)
	logger.Info("exporting")

	// export changed pull requests
	var pullRequestsErr error
	pullRequestsInitial := make(chan []api.PullRequest)
	go func() {
		defer close(pullRequestsInitial)
		err := s.exportPullRequestsRepo(logger, repo, pullRequestsInitial, pullRequestSender.LastProcessed)
		if err != nil {
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
		err := s.exportPullRequestsComments(logger, commentsSender, repo, pullRequestsForComments)
		if err != nil {
			setErr(fmt.Errorf("error getting comments %s", err))
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.exportPullRequestsReviews(logger, reviewSender, repo, pullRequestsForReviews)
		if err != nil {
			setErr(fmt.Errorf("error getting reviews %s", err))
		}
	}()

	// set commits on the rp and then send the pr
	wg.Add(1)
	go func() {
		defer wg.Done()
		for prs := range pullRequestsForCommits {
			for _, pr := range prs {
				commits, err := s.exportPullRequestCommits(logger, repo.ID, pr.IID)
				if err != nil {
					setErr(fmt.Errorf("error getting commits %s", err))
					return
				}
				pr.CommitShas = commits
				pr.CommitIds = ids.CodeCommits(s.qc.CustomerID, s.refType, pr.RepoID, commits)
				if len(pr.CommitShas) == 0 {
					logger.Info("found PullRequest with no commits (ignoring it)", "repo", repo.NameWithOwner, "pr_ref_id", pr.RefID, "pr.url", pr.URL)
				} else {
					pr.BranchID = s.qc.BasicInfo.BranchID(pr.RepoID, pr.BranchName, pr.CommitShas[0])
				}
				err = pullRequestSender.Send(pr)
				if err != nil {
					setErr(err)
					return
				}
			}
		}
	}()
	wg.Wait()
	return
}
