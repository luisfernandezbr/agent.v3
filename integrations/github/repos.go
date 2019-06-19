package main

import (
	"context"

	"github.com/pinpt/agent2/rpcdef"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func (s *Integration) exportRepos(ctx context.Context) error {
	refType := "sourcecode.repo"
	sessionID := s.agent.ExportStarted(refType)
	defer s.agent.ExportDone(sessionID)

	return paginate(func(cursors []string) (nextCursors []string, _ error) {
		return s.exportReposPageFrom(sessionID, cursors)
	})
}

func (s *Integration) exportReposPageFrom(sessionID string, cursors []string) (nextCursor []string, _ error) {
	refType := "sourcecode.repo"

	query := `
	query {
		viewer {
			organization(login:"pinpt"){
				repositories(first:100 ` + makeAfterParam(cursors, 0) + `) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
					}
					nodes {
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
						TotalCount int `json:"totalCount"`
						PageInfo   struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
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

	// TODO: update session progress using total count

	err := s.makeRequest(query, &res)
	if err != nil {
		return nil, err
	}

	repositories := res.Data.Viewer.Organization.Repositories
	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		s.logger.Warn("no repos found")
		return nil, nil
	}

	batch := []rpcdef.ExportObj{}
	for _, data := range repoNodes {
		repo := sourcecode.Repo{}
		repo.CustomerID = s.customerID
		repo.RefType = refType
		repo.RefID = data.ID
		repo.Name = data.Name
		repo.URL = data.URL
		batch = append(batch, rpcdef.ExportObj{Data: repo.ToMap()})
	}

	hasMore := repositories.PageInfo.HasNextPage
	lastCursor := repositories.PageInfo.EndCursor
	s.agent.SendExported(sessionID, lastCursor, batch)

	if hasMore {
		return []string{lastCursor}, nil
	}
	return nil, nil
}
