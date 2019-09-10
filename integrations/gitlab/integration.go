package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/pkg/commitusers"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type Config struct {
	URL           string   `json:"url"`
	APIToken      string   `json:"api_token"`
	ExcludedRepos []string `json:"excluded_repos"`
	Repos         []string `json:"repos"`
	StopAfterN    int      `json:"stop_after_n"`
	OnlyGit       bool     `json:"only_git"`
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc api.QueryContext

	config Config

	requestConcurrencyChan chan bool

	refType string

	repoSender       *objsender.IncrementalDateBased
	commitUserSender *objsender.IncrementalDateBased
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
	exportConfig rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	err := s.initWithConfig(exportConfig)
	if err != nil {
		return res, err
	}

	err = s.export(ctx)
	if err != nil {
		return res, err
	}

	return res, nil
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
		requester := api.NewRequester(opts)

		s.qc.Request = requester.Request
		s.qc.RequestGraphQL = requester.RequestGraphQL
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

	s.qc.UserEmailMap, err = api.UserEmailMap(s.qc)
	if err != nil {
		return err
	}

	groupNames, err := api.Groups(s.qc)
	if err != nil {
		return err
	}

	for _, groupName := range groupNames {
		if err := s.exportGroup(ctx, groupName); err != nil {
			return nil
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

	return
}

func (s *Integration) exportGroup(ctx context.Context, groupName string) error {
	s.logger.Info("exporting group", "name", groupName)
	logger := s.logger.With("org", groupName)

	repos, err := api.ReposAllSlice(s.qc, groupName)
	if err != nil {
		return err
	}

	repos = s.filterRepos(logger, repos)

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
			args.RepoID = s.qc.RepoID(repo.ID)
			args.URL = repoURL
			args.CommitURLTemplate = commitURLTemplate(repo, s.config.URL)
			args.BranchURLTemplate = branchURLTemplate(repo, s.config.URL)
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

	// export a link between commit and github user
	// This is much slower than the rest
	// for pinpoint takes 3.5m for initial, 47s for incremental
	{
		// higher concurrency does not make any real difference
		commitConcurrency := 1

		err := s.exportCommitUsers(logger, repos, commitConcurrency)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Integration) filterRepos(logger hclog.Logger, repos []api.Repo) (res []api.Repo) {
	if len(s.config.Repos) != 0 {
		ok := map[string]bool{}
		for _, nameWithOwner := range s.config.Repos {
			ok[nameWithOwner] = true
		}
		for _, repo := range repos {
			if !ok[repo.NameWithOwner] {
				continue
			}
			res = append(res, repo)
		}
		logger.Info("repos", "found", len(repos), "repos_specified", len(s.config.Repos), "result", len(res))
		return
	}

	//all := map[string]bool{}
	//for _, repo := range repos {
	//	all[repo.ID] = true
	//}
	excluded := map[string]bool{}
	for _, id := range s.config.ExcludedRepos {
		// This does not work because we pass excluded repos for all orgs. But filterRepos is called separately per org.
		//if !all[id] {
		//	return fmt.Errorf("wanted to exclude non existing repo: %v", id)
		//}
		excluded[id] = true
	}

	filtered := map[string]api.Repo{}
	// filter excluded repos
	for _, repo := range repos {
		if excluded[repo.ID] {
			continue
		}
		filtered[repo.ID] = repo
	}

	logger.Info("repos", "found", len(repos), "excluded_definition", len(s.config.ExcludedRepos), "result", len(filtered))
	for _, repo := range filtered {
		res = append(res, repo)
	}
	return
}

func commitURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/commit/@@@sha@@@"
}

func branchURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/tree/@@@branch@@@"
}

func urlAppend(p1, p2 string) string {
	return strings.TrimSuffix(p1, "/") + "/" + p2
}

func (s *Integration) exportRepos(ctx context.Context, logger hclog.Logger, groupName string, onlyInclude []api.Repo) error {

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
