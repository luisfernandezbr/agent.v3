package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/pinpt/go-datamodel/sourcecode"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent2/rpcdef"
	"github.com/pinpt/go-common/hash"
)

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	s.customerID = "c1"
	return nil
}

func (s *Integration) Export(ctx context.Context) error {
	s.logger.Info("TODO: export called, needs implementing")
	err := s.exportRepos(ctx)
	if err != nil {
		return err
	}
	err = s.exportPullRequests(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *Integration) exportRepos(ctx context.Context) error {
	refType := "sourcecode.repo"
	sessionID := s.agent.ExportStarted(refType)
	defer s.agent.ExportDone(sessionID)

	query := `query {
		viewer {
			organization(login:"pinpt"){
				repositories(last:100) {
					nodes {
						id
						name
						url
					}
				}
			}
		}
	}`

	var res struct {
		Data struct {
			Viewer struct {
				Organization struct {
					Repositories struct {
						Nodes []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
							URL  string `json:"url"`
						} `json:"nodes"`
					} `json:"repositories"`
				} `json:"organization"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := s.makeRequest(query, &res)
	if err != nil {
		return err
	}

	batch := []rpcdef.ExportObj{}

	repoNodes := res.Data.Viewer.Organization.Repositories.Nodes
	s.logger.Info("retrieved repos", "n", len(repoNodes))
	for _, data := range repoNodes {
		repo := sourcecode.Repo{}
		repo.CustomerID = s.customerID
		repo.RefType = refType
		repo.RefID = data.ID
		repo.Name = data.Name
		repo.URL = data.URL
		batch = append(batch, rpcdef.ExportObj{Data: repo.ToMap()})
		if len(batch) >= batchSize {
			s.agent.SendExported(sessionID, "todo_last_processed", batch)
			batch = []rpcdef.ExportObj{}
		}
	}
	if len(batch) != 0 {
		s.agent.SendExported(sessionID, "todo_last_processed", batch)
	}

	return nil
}

func (s *Integration) exportPullRequests(ctx context.Context) error {
	refType := "sourcecode.pull_request"
	sessionID := s.agent.ExportStarted(refType)
	defer s.agent.ExportDone(sessionID)

	query := `
	query { 
		viewer { 
			organization(login:"pinpt"){
				repositories(last:10) {
					nodes {
						pullRequests(last:10) {
							nodes {
								id
								repository { id }
								title
								bodyText
								url
								createdAt
								mergedAt
								closedAt
								updatedAt
								# OPEN, CLOSED or MERGED
								state
								author { login }
							}
						}
					}
				}
			}
		}
	}
	`

	var res struct {
		Data struct {
			Viewer struct {
				Organization struct {
					Repositories struct {
						Nodes []struct {
							PullRequests struct {
								Nodes []struct {
									ID         string `json:"id"`
									Repository struct {
										ID string `json:"id"`
									}
									Title    string `json:"title"`
									BodyText string `json:"bodyText"`

									URL       string `json:"url"`
									CreatedAt string `json:"createdAt"`
									MergedAt  string `json:"mergedAt"`
									ClosedAt  string `json:"closedAt"`
									UpdatedAt string `json:"updatedAt"`
									State     string `json:"state"`
									Author    struct {
										Login string `json:"login"`
									}
								} `json:"nodes"`
							} `json:"pullRequests"`
						} `json:"nodes"`
					} `json:"repositories"`
				} `json:"organization"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := s.makeRequest(query, &res)
	if err != nil {
		return err
	}

	batch := []rpcdef.ExportObj{}

	c := 0
	repoNodes := res.Data.Viewer.Organization.Repositories.Nodes
	for _, repoNode := range repoNodes {
		pullRequestNodes := repoNode.PullRequests.Nodes
		c += len(pullRequestNodes)
	}
	s.logger.Info("retrieved pull requests", "n", c)

	for _, repoNode := range repoNodes {
		pullRequestNodes := repoNode.PullRequests.Nodes
		for _, node := range pullRequestNodes {
			pr := sourcecode.PullRequest{}
			pr.CustomerID = s.customerID
			pr.RefType = refType
			pr.RefID = node.ID
			pr.RepoID = hash.Values("Repo", s.customerID, "sourcecode.Repo", node.Repository.ID)
			pr.Title = node.Title
			pr.Description = node.BodyText
			pr.URL = node.URL
			pr.CreatedAt = parseTime(node.CreatedAt)
			pr.MergedAt = parseTime(node.MergedAt)
			pr.ClosedAt = parseTime(node.ClosedAt)
			pr.UpdatedAt = parseTime(node.UpdatedAt)
			validStatus := []string{"OPEN", "CLOSED", "MERGED"}
			if !strInArr(node.State, validStatus) {
				panic("unknown state: " + node.State)
			}
			pr.Status = node.State
			pr.UserRefID = hash.Values("User", s.customerID, "sourcecode.User", node.Author.Login)
			batch = append(batch, rpcdef.ExportObj{Data: pr.ToMap()})
			if len(batch) >= batchSize {
				s.agent.SendExported(sessionID, "todo_last_processed", batch)
				batch = []rpcdef.ExportObj{}
			}
		}
	}
	if len(batch) != 0 {
		s.agent.SendExported(sessionID, "todo_last_processed", batch)
	}

	return nil
}

func strInArr(str string, arr []string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

func parseTime(ts string) int64 {
	if ts == "" {
		return 0
	}
	ts2, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		panic(err)
	}
	return ts2.Unix()
}

const batchSize = 100

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

	//s.logger.Info("response body", string(b))

	err = json.Unmarshal(b, &res)
	if err != nil {
		return err
	}
	return nil
}
