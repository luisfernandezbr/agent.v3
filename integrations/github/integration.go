package main

import (
	"context"
	"net/url"
	"os"
	"sync"

	"github.com/pinpt/go-common/hash"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
)

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc    api.QueryContext
	users *Users

	config Config

	requestConcurrencyChan chan bool
}

// setting higher to 1 starts returning the followign error, even though the hourly limit is not used up yet.
// 403: You have triggered an abuse detection mechanism. Please wait a few minutes before you try again.
const maxRequestConcurrency = 1

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	s.customerID = "c1"

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
	s.readConfig()
	s.requestConcurrencyChan = make(chan bool, maxRequestConcurrency)

	return nil
}

type Config struct {
	APIURL        string
	RepoURLPrefix string
	Token         string
	Org           string
}

func (s *Integration) readConfig() error {
	conf := Config{}
	conf.APIURL = "https://api.github.com/graphql"
	conf.RepoURLPrefix = "https://github.com"

	token := os.Getenv("PP_GITHUB_TOKEN")
	if token == "" {
		panic("provide PP_GITHUB_TOKEN")
	}
	conf.Token = token

	org := os.Getenv("PP_GITHUB_ORG")
	if org == "" {
		panic("provide PP_GITHUB_ORG")
	}
	conf.Org = org

	s.config = conf
	return nil
}

func (s *Integration) Export(ctx context.Context) error {

	// export all users in organization, and when later encountering new users continue export
	var err error
	s.users, err = NewUsers(s)
	if err != nil {
		return err
	}
	defer s.users.Done()

	s.qc.UserLoginToRefID = s.users.LoginToRefID
	s.qc.UserLoginToRefIDFromCommit = s.users.LoginToRefIDFromCommit

	repos, err := api.ReposAllSlice(s.qc)
	if err != nil {
		return err
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

	// export repos
	{
		err := s.exportRepos(ctx)
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
