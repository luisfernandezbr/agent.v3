package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func (s *Integration) exportRepos(ctx context.Context) error {
	refType := "sourcecode.repo"
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

	return paginate(lastProcessed, func(query string, stopOnUpdatedAt time.Time) (pageInfo, error) {
		return s.exportReposPageFrom(sessionID, query, stopOnUpdatedAt)
	})
}

func (s *Integration) exportReposPageFrom(sessionID string, queryParams string, stopOnUpdatedAt time.Time) (pageInfo, error) {
	s.logger.Info("export repos request", "q", queryParams)

	refType := "sourcecode.repo"

	query := `
	query {
		viewer {
			organization(login:"pinpt"){
				repositories(` + queryParams + `) {
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
						name
						url						
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
						TotalCount int      `json:"totalCount"`
						PageInfo   pageInfo `json:"pageInfo"`
						Nodes      []struct {
							UpdatedAt time.Time `json:"updatedAt"`
							ID        string    `json:"id"`
							Name      string    `json:"name"`
							URL       string    `json:"url"`
						} `json:"nodes"`
					} `json:"repositories"`
				} `json:"organization"`
			} `json:"viewer"`
		} `json:"data"`
	}

	// TODO: update session progress using total count

	err := s.makeRequest(query, &res)
	if err != nil {
		return pageInfo{}, err
	}

	repositories := res.Data.Viewer.Organization.Repositories
	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		s.logger.Warn("no repos found")
		return pageInfo{}, nil
	}

	batch := []rpcdef.ExportObj{}

	sendResults := func() {
		s.agent.SendExported(sessionID, batch)
	}

	for _, data := range repoNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			if len(batch) != 0 {
				sendResults()
			}
			return pageInfo{}, nil
		}
		repo := sourcecode.Repo{}
		repo.CustomerID = s.customerID
		repo.RefType = refType
		repo.RefID = data.ID
		repo.Name = data.Name
		repo.URL = data.URL
		batch = append(batch, rpcdef.ExportObj{Data: repo.ToMap()})
	}

	sendResults()

	return repositories.PageInfo, nil
}
