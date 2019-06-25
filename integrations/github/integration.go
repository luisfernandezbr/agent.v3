package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
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
	s.qc = qc

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

	// export repos
	{
		err := s.exportRepos(ctx)
		if err != nil {
			return err
		}
	}

	// get all repo ids
	repoIDs := make(chan []string)
	go func() {
		defer close(repoIDs)
		err := api.ReposAllIDs(s.qc, repoIDs)
		if err != nil {
			panic(err)
		}
	}()

	// at the same time, export updated pull requests
	pullRequests := make(chan []api.PullRequest)
	go func() {
		defer close(pullRequests)
		err := s.exportPullRequests(repoIDs, pullRequests)
		if err != nil {
			panic(err)
		}
	}()

	//for range pullRequests {
	//}
	//return nil

	pullRequestsForComments := make(chan []api.PullRequest)
	pullRequestsForReviews := make(chan []api.PullRequest)

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

func (s *Integration) makeRequest(query string, res interface{}) error {
	data := map[string]string{
		"query": query,
	}

	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewReader(b))
	if err != nil {
		return err
	}
	auth := os.Getenv("PP_GITHUB_TOKEN")
	if auth == "" {
		return errors.New("provide PP_GITHUB_TOKEN")
	}
	req.Header.Add("Authorization", "bearer "+auth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	//TODO: catch errors properly here
	// example {"errors":[{"message"...}]
	if resp.StatusCode != 200 {

		return errors.New(`resp resp.StatusCode != 200`)
	}

	//s.logger.Info("response body", string(b))

	err = json.Unmarshal(b, &res)
	if err != nil {
		return err
	}
	return nil
}
