package main

import (
	"fmt"
	"time"

	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func (s *Integration) exportPullRequests(repoIDs chan []string) error {
	refType := "sourcecode.pull_requests"
	sessionID, lastProcessedData := s.agent.ExportStarted(refType)
	defer s.agent.ExportDone(sessionID, time.Now().Format(time.RFC3339))

	var lastProcessed time.Time
	if lastProcessedData != nil {
		var err error
		lastProcessed, err = time.Parse(time.RFC3339, lastProcessedData.(string))
		if err != nil {
			return fmt.Errorf("last processed timestamp is not valid, err: %v", err)
		}
	}

	for ids := range repoIDs {
		for _, id := range ids {
			err := s.exportPullRequestsRepo(id, sessionID, lastProcessed)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestsRepo(repoID string, sessionID string, lastProcessed time.Time) error {
	return paginate(lastProcessed, func(query string, stopOnUpdatedAt time.Time) (pageInfo, error) {
		return s.exportPullRequestsPageFrom(sessionID, repoID, query, stopOnUpdatedAt)
	})
}

func (s *Integration) exportPullRequestsPageFrom(sessionID string, repoID string, queryParams string, stopOnUpdatedAt time.Time) (pi pageInfo, _ error) {
	s.logger.Info("export pull_request request", "repo", repoID, "q", queryParams)

	refType := "sourcecode.pull_request"

	query := `
	query {
		node (id: "` + repoID + `") {
			... on Repository {
				pullRequests(` + queryParams + `) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
						hasPreviousPage
						startCursor
					}
					nodes {
						updatedAt
						id
						repository { id }
						title
						bodyText
						url
						createdAt
						mergedAt
						closedAt
						# OPEN, CLOSED or MERGED
						state
						author { login }
					}
				}
			}
		}
	}
	`

	var res struct {
		Data struct {
			Node struct {
				PullRequests struct {
					TotalCount int      `json:"totalCount"`
					PageInfo   pageInfo `json:"pageInfo"`
					Nodes      []struct {
						ID         string `json:"id"`
						Repository struct {
							ID string `json:"id"`
						}
						Title    string `json:"title"`
						BodyText string `json:"bodyText"`

						URL       string    `json:"url"`
						CreatedAt time.Time `json:"createdAt"`
						MergedAt  time.Time `json:"mergedAt"`
						ClosedAt  time.Time `json:"closedAt"`
						UpdatedAt time.Time `json:"updatedAt"`
						State     string    `json:"state"`
						Author    struct {
							Login string `json:"login"`
						}
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"node"`
		} `json:"data"`
	}

	err := s.makeRequest(query, &res)
	if err != nil {
		return pi, err
	}

	//s.logger.Info(fmt.Sprintf("%+v", res))

	batch := []rpcdef.ExportObj{}

	pullRequestNodes := res.Data.Node.PullRequests.Nodes
	s.logger.Info("retrieved pull requests", "n", len(pullRequestNodes))

	sendResults := func() {
		s.agent.SendExported(sessionID, batch)
	}

	for _, data := range pullRequestNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			if len(batch) != 0 {
				sendResults()
			}
			return pageInfo{}, nil
		}

		pr := sourcecode.PullRequest{}
		pr.CustomerID = s.customerID
		pr.RefType = refType
		pr.RefID = data.ID
		pr.RepoID = hash.Values("Repo", s.customerID, "sourcecode.Repo", data.Repository.ID)
		pr.Title = data.Title
		pr.Description = data.BodyText
		pr.URL = data.URL
		pr.CreatedAt = data.CreatedAt.Unix()
		pr.MergedAt = data.MergedAt.Unix()
		pr.ClosedAt = data.ClosedAt.Unix()
		pr.UpdatedAt = data.UpdatedAt.Unix()
		validStatus := []string{"OPEN", "CLOSED", "MERGED"}
		if !strInArr(data.State, validStatus) {
			panic("unknown state: " + data.State)
		}
		pr.Status = data.State
		pr.UserRefID = hash.Values("User", s.customerID, "sourcecode.User", data.Author.Login)

		batch = append(batch, rpcdef.ExportObj{Data: pr.ToMap()})
	}

	sendResults()

	return res.Data.Node.PullRequests.PageInfo, nil
}
