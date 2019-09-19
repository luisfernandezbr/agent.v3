package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/bitbucket/api"
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

type Config struct {
	URL                string   `json:"url"`
	Username           string   `json:"username"`
	Password           string   `json:"password"`
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

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	s.refType = "bitbucket"

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
	s.commonInfo = commonrepo.Config{
		Repos:         s.config.Repos,
		ExcludedRepos: s.config.ExcludedRepos,
		StopAfterN:    s.config.StopAfterN,
	}

	{
		opts := api.RequesterOpts{}
		opts.Logger = s.logger
		opts.APIURL = s.config.URL + "/2.0"
		opts.Username = s.config.Username
		opts.Password = s.config.Password
		opts.InsecureSkipVerify = s.config.InsecureSkipVerify
		requester := api.NewRequester(opts)

		s.qc.Request = requester.Request
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
	var def Config
	err := structmarshal.MapToStruct(data, &def)
	if err != nil {
		return err
	}
	if def.URL == "" {
		return rerr("url is missing")
	}
	if def.Username == "" {
		return rerr("username is missing")
	}
	if def.Password == "" {
		return rerr("password is missing")
	}

	s.config = def
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

	teamNames, err := api.Teams(s.qc)
	if err != nil {
		return err
	}

	for _, teamName := range teamNames {
		if err := s.exportTeam(ctx, teamName); err != nil {
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

func (s *Integration) exportTeam(ctx context.Context, groupName string) error {
	s.logger.Info("exporting group", "name", groupName)
	logger := s.logger.With("org", groupName)

	repos, err := commonrepo.ReposAllSlice(s.qc, groupName, func(res chan []commonrepo.Repo) error {
		return api.ReposAll(s.qc, groupName, res)
	})
	if err != nil {
		return err
	}

	repos = commonrepo.FilterRepos(logger, repos, s.commonInfo)

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
			u.User = url.UserPassword(s.config.Username, s.config.Password)
			u.Path = "/" + repo.NameWithOwner
			repoURL := u.Scheme + "://" + u.User.String() + "@" + api.GetDomain(u.Host) + u.EscapedPath()

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
	{
		err := s.exportRepos(ctx, logger, groupName, repos)
		if err != nil {
			return err
		}
	}

	// export users
	{
		err := s.exportUsers(ctx, logger, groupName)
		if err != nil {
			return err
		}
	}

	// export repos
	{
		err := s.exportCommitUsers(ctx, logger, repos)
		if err != nil {
			return err
		}
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

	return api.PaginateNewerThan(s.logger, sender.LastProcessed, func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, repos, err := api.ReposSourcecodePage(s.qc, groupName, parameters, stopOnUpdatedAt)
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
}

func (s *Integration) exportUsers(ctx context.Context, logger hclog.Logger, groupName string) error {

	sender := s.userSender

	return api.Paginate(s.logger, func(log hclog.Logger, parameters url.Values) (api.PageInfo, error) {
		pi, users, err := api.UsersSourcecodePage(s.qc, groupName, parameters)
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

func (s *Integration) exportCommitUsers(ctx context.Context, logger hclog.Logger, repos []commonrepo.Repo) (err error) {

	sender := s.commitUserSender

	for _, repo := range repos {
		err = api.Paginate(s.logger, func(log hclog.Logger, parameters url.Values) (api.PageInfo, error) {
			pi, users, err := api.CommitUsersSourcecodePage(s.qc, repo.NameWithOwner, parameters)
			if err != nil {
				return pi, err
			}
			for _, user := range users {
				err := sender.SendMap(user.ToMap())
				if err != nil {
					return pi, err
				}
			}
			return pi, nil
		})
		if err != nil {
			return
		}
	}

	return
}
