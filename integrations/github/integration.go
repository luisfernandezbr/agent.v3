package main

import (
	"context"
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
}

type Config struct {
	Token string
	Org   string
}

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

	{
		token := os.Getenv("PP_GITHUB_TOKEN")
		if token == "" {
			panic("provide PP_GITHUB_TOKEN")
		}
		s.config.Token = token

		org := os.Getenv("PP_GITHUB_ORG")
		if org == "" {
			panic("provide PP_GITHUB_ORG")
		}
		s.config.Org = org

	}
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
	pullRequests := make(chan []api.PullRequest, 1000)
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

	pullRequestsForComments := make(chan []api.PullRequest, 1000)
	pullRequestsForReviews := make(chan []api.PullRequest, 1000)

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
