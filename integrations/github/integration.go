package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/pinpt/agent.next/pkg/structmarshal"

	"github.com/pinpt/go-common/hash"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/rpcdef"
)

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc    api.QueryContext
	users *Users

	config Config

	requestConcurrencyChan chan bool
}

func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}

// setting higher to 1 starts returning the following error, even though the hourly limit is not used up yet.
// 403: You have triggered an abuse detection mechanism. Please wait a few minutes before you try again.
const maxRequestConcurrency = 1

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent

	qc := api.QueryContext{}
	qc.Logger = s.logger
	qc.Request = s.makeRequest
	qc.RepoID = func(refID string) string {
		return hash.Values("Repo", s.customerID, "github", refID)
	}
	qc.UserID = func(refID string) string {
		return hash.Values("User", s.customerID, "github", refID)
	}
	qc.PullRequestID = func(refID string) string {
		return hash.Values("PullRequest", s.customerID, "github", refID)
	}
	qc.IsEnterprise = func() bool {
		return s.config.Enterprise
	}
	s.qc = qc
	s.requestConcurrencyChan = make(chan bool, maxRequestConcurrency)

	return nil
}

type Config struct {
	APIURL        string
	APIURL3       string
	RepoURLPrefix string
	Token         string
	OnlyOrg       string
	ExcludedRepos []string
	OnlyGit       bool
	StopAfterN    int
	Enterprise    bool
}

type configDef struct {
	URL           string   `json:"url"`
	APIToken      string   `json:"apitoken"`
	ExcludedRepos []string `json:"excluded_repos"`
	OnlyGit       bool     `json:"only_git"`
	// OnlyOrganization specifies the organization to export. By default all account organization are exported. Set this to export only one.
	OnlyOrganization string `json:"only_organization"`
	// StopAfterN stops exporting after N number of repos for testing and dev purposes
	StopAfterN int `json:"stop_after_n"`
}

func (s *Integration) setIntegrationConfig(data map[string]interface{}) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}
	var def configDef
	err := structmarshal.MapToStruct(data, &def)
	if err != nil {
		return err
	}

	if def.URL == "" {
		return rerr("url is missing")
	}
	if def.APIToken == "" {
		return rerr("apitoken is missing")
	}
	var res Config
	res.Token = def.APIToken
	res.OnlyOrg = def.OnlyOrganization
	res.ExcludedRepos = def.ExcludedRepos
	res.OnlyGit = def.OnlyGit
	res.StopAfterN = def.StopAfterN

	{
		u, err := url.Parse(def.URL)
		if err != nil {
			return rerr("url is invalid: %v", err)
		}
		// allow both http(s)://github.com and http(s)://api.github.com
		if u.Host == "github.com" || u.Host == "api.github.com" {
			u, _ = url.Parse("https://api.github.com")
		}
		if u.Host == "api.github.com" {
			res.APIURL = urlAppend(u.String(), "graphql")
			res.RepoURLPrefix = "https://" + strings.TrimPrefix(u.Host, "api.")
		} else {
			res.APIURL = urlAppend(u.String(), "api/graphql")
			res.APIURL3 = urlAppend(u.String(), "api/v3")
			res.RepoURLPrefix = u.String()
			res.Enterprise = true
		}

	}

	s.config = res

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

	orgs, err := s.getOrgs()
	if err != nil {
		rerr(err)
		return
	}

	_, err = api.ReposAllSlice(s.qc, orgs[0])
	if err != nil {
		rerr(err)
		return
	}

	// TODO: return a repo and validate repo that repo can be cloned in agent

	return
}

func urlAppend(p1, p2 string) string {
	return strings.TrimSuffix(p1, "/") + "/" + p2
}

func (s *Integration) initWithConfig(exportConfig rpcdef.ExportConfig) error {
	s.customerID = exportConfig.Pinpoint.CustomerID
	s.qc.CustomerID = s.customerID
	err := s.setIntegrationConfig(exportConfig.Integration)
	if err != nil {
		return err
	}
	s.qc.APIURL3 = s.config.APIURL3

	return nil
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

func (s *Integration) getOrgs() (res []api.Org, _ error) {
	if s.config.OnlyOrg != "" {
		s.logger.Info("only_organization passed", "org", s.config.OnlyOrg)
		return []api.Org{{Login: s.config.OnlyOrg}}, nil
	}
	var err error
	if !s.config.Enterprise {
		res, err = api.OrgsAll(s.qc)
		if err != nil {
			return nil, err
		}
	} else {
		res, err = api.OrgsEnterpriseAll(s.qc)
		if err != nil {
			return nil, err
		}
	}
	if len(res) == 0 {
		return nil, errors.New("no organizations found in account")
	}
	var names []string
	for _, org := range res {
		names = append(names, org.Login)

	}
	s.logger.Info("found organizations", "orgs", names)
	return res, nil
}

func (s *Integration) export(ctx context.Context) error {
	orgs, err := s.getOrgs()
	if err != nil {
		return err
	}

	// export all users in all organization, and when later encountering new users continue export
	s.users, err = NewUsers(s, orgs)
	if err != nil {
		return err
	}

	s.qc.UserLoginToRefID = s.users.LoginToRefID
	s.qc.UserLoginToRefIDFromCommit = s.users.LoginToRefIDFromCommit

	for _, org := range orgs {
		err := s.exportOrganization(ctx, org)
		if err != nil {
			return err
		}
	}

	return s.users.Done()
}

func commitURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/commit/@@@sha@@@"
}

func (s *Integration) exportOrganization(ctx context.Context, org api.Org) error {
	s.logger.Info("exporting organization", "login", org.Login)
	repos, err := api.ReposAllSlice(s.qc, org)
	if err != nil {
		return err
	}

	{
		//all := map[string]bool{}
		//for _, repo := range repos {
		//	all[repo.ID] = true
		//}
		excluded := map[string]bool{}
		for _, id := range s.config.ExcludedRepos {
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

		s.logger.Info("repos", "found", len(repos), "excluded_definition", len(s.config.ExcludedRepos), "result", len(filtered))
		repos = []api.Repo{}
		for _, repo := range filtered {
			repos = append(repos, repo)
		}
	}

	if s.config.StopAfterN != 0 {
		// only leave 1 repo for export
		stopAfter := s.config.StopAfterN
		l := len(repos)
		if len(repos) > stopAfter {
			repos = repos[0:stopAfter]
		}
		s.logger.Info("stop_after_n passed", "v", stopAfter, "repos", l, "after", len(repos))
	}

	// queue repos for processing with ripsrc
	{

		for _, repo := range repos {
			u, err := url.Parse(s.config.RepoURLPrefix)
			if err != nil {
				return err
			}
			u.User = url.UserPassword(s.config.Token, "")
			u.Path = repo.NameWithOwner
			repoURL := u.String()

			args := rpcdef.GitRepoFetch{}
			args.RepoID = s.qc.RepoID(repo.ID)
			args.URL = repoURL
			args.CommitURLTemplate = commitURLTemplate(repo, s.config.RepoURLPrefix)
			s.agent.ExportGitRepo(args)
		}
	}

	if s.config.OnlyGit {
		s.logger.Warn("only_ripsrc flag passed, skipping export of data from github api")
		return nil
	}

	// export repos
	{
		err := s.exportRepos(ctx, org, s.config.ExcludedRepos)
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

		err := s.exportCommitUsers(repos, commitConcurrency)
		if err != nil {
			return err
		}
	}

	// at the same time, export updated pull requests
	pullRequests := make(chan []api.PullRequest, 10)
	go func() {
		defer close(pullRequests)
		err := s.exportPullRequests(repos, pullRequests)
		if err != nil {
			panic(err)
		}
	}()

	//for range pullRequests {
	//}
	//return nil

	pullRequestsForComments := make(chan []api.PullRequest, 10)
	pullRequestsForReviews := make(chan []api.PullRequest, 10)

	go func() {
		for item := range pullRequests {
			pullRequestsForComments <- item
			pullRequestsForReviews <- item
		}
		close(pullRequestsForComments)
		close(pullRequestsForReviews)
	}()

	wg := sync.WaitGroup{}
	wg.Add(2)

	// at the same time, export all comments for updated pull requests
	go func() {
		defer wg.Done()
		err := s.exportPullRequestComments(pullRequestsForComments)
		if err != nil {
			panic(err)
		}
	}()
	// at the same time, export all reviews for updated pull requests
	go func() {
		defer wg.Done()
		err := s.exportPullRequestReviews(pullRequestsForReviews)
		if err != nil {
			panic(err)
		}
	}()
	wg.Wait()
	return nil
}
