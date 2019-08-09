package main

import (
	"context"
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
	qc.CustomerID = s.customerID
	qc.RepoID = func(refID string) string {
		return hash.Values("Repo", s.customerID, "sourcecode.Repo", refID)
	}
	qc.UserID = func(refID string) string {
		return hash.Values("User", s.customerID, "sourcecode.User", refID)
	}
	qc.PullRequestID = func(refID string) string {
		return hash.Values("PullRequest", s.customerID, "sourcecode.PullRequest", refID)
	}
	qc.Organization = func() string {
		return s.config.Org
	}
	s.qc = qc
	s.requestConcurrencyChan = make(chan bool, maxRequestConcurrency)

	return nil
}

type Config struct {
	APIURL        string
	RepoURLPrefix string
	Token         string
	Org           string
	ExcludedRepos []string
	OnlyRipsrc    bool
}

type configDef struct {
	URL           string   `json:"url"`
	APIToken      string   `json:"apitoken"`
	Organization  string   `json:"organization"`
	ExcludedRepos []string `json:"excluded_repos"`
	OnlyRipsrc    bool     `json:"only_ripsrc"`
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
	if def.Organization == "" {
		return rerr("organization is missing")
	}

	var res Config
	res.Token = def.APIToken
	res.Org = def.Organization
	res.ExcludedRepos = def.ExcludedRepos
	res.OnlyRipsrc = def.OnlyRipsrc

	apiURLBaseParsed, err := url.Parse(def.URL)
	if err != nil {
		return rerr("url is invalid: %v", err)
	}
	res.APIURL = urlAppend(def.URL, "graphql")
	res.RepoURLPrefix = "https://" + strings.TrimPrefix(apiURLBaseParsed.Host, "api.")

	s.config = res
	return nil
}

func urlAppend(p1, p2 string) string {
	return strings.TrimSuffix(p1, "/") + "/" + p2
}

func (s *Integration) Export(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {

	s.customerID = exportConfig.Pinpoint.CustomerID
	err := s.setIntegrationConfig(exportConfig.Integration)
	if err != nil {
		return res, err
	}

	err = s.export(ctx)
	if err != nil {
		return res, err
	}

	return res, nil
}

func (s *Integration) export(ctx context.Context) error {

	repos, err := api.ReposAllSlice(s.qc)
	if err != nil {
		return err
	}
	{
		excluded := map[string]bool{}
		for _, id := range s.config.ExcludedRepos {
			excluded[id] = true
		}
		var filtered []api.Repo
		// filter excluded repos
		for _, repo := range repos {
			if excluded[repo.ID] {
				continue
			}
			filtered = append(filtered, repo)
		}
		s.logger.Info("repos", "found", len(repos), "excluded_definition", len(s.config.ExcludedRepos), "result", len(filtered))
		repos = filtered
	}

	// queue repos for processing with ripsrc
	{

		for _, repo := range repos {
			u, err := url.Parse(s.config.RepoURLPrefix)
			if err != nil {
				return err
			}
			u.User = url.UserPassword(s.config.Token, "")
			u.Path = s.config.Org + "/" + repo.Name
			repoURL := u.String()

			args := rpcdef.GitRepoFetch{}
			args.RepoID = s.qc.RepoID(repo.ID)
			args.URL = repoURL
			s.agent.ExportGitRepo(args)
		}
	}

	if s.config.OnlyRipsrc {
		s.logger.Warn("only_ripsrc flag passed, skipping export of data from github api")
		return nil
	}

	// export all users in organization, and when later encountering new users continue export
	s.users, err = NewUsers(s)
	if err != nil {
		return err
	}
	defer s.users.Done()

	s.qc.UserLoginToRefID = s.users.LoginToRefID
	s.qc.UserLoginToRefIDFromCommit = s.users.LoginToRefIDFromCommit

	// export repos
	{
		err := s.exportRepos(ctx, s.config.ExcludedRepos)
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
