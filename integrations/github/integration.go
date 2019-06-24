package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pinpt/go-common/hash"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
)

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc api.QueryContext
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

	// export repos
	{
		err := s.exportRepos(ctx)
		if err != nil {
			return err
		}
	}

	// export users
	{
		err := s.exportUsers(ctx)
		if err != nil {
			return err
		}
	}

	// get all repo ids
	repoIDs := make(chan []string)
	go func() {
		//return
		defer close(repoIDs)
		err := api.ReposAllIDs(s.qc, repoIDs)
		if err != nil {
			panic(err)
		}
	}()

	// at the same time, export updated pull requests
	pullRequests := make(chan []api.PullRequest)
	go func() {
		//return

		defer close(pullRequests)
		err := s.exportPullRequests(repoIDs, pullRequests)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		//pullRequestIDs <- []string{"MDExOlB1bGxSZXF1ZXN0MjkxMjAxMDk5"}
		//close(pullRequestIDs)
	}()

	// at the same time, export all comments for updated pull requests
	{
		err := s.exportPullRequestComments(pullRequests)
		if err != nil {
			panic(err)
		}
	}

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
