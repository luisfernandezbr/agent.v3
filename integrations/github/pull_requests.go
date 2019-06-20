package main

import (
	"context"

	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func (s *Integration) exportPullRequests(ctx context.Context) error {
	refType := "sourcecode.pull_requests"
	sessionID := s.agent.ExportStarted(refType)
	defer s.agent.ExportDone(sessionID)

	return paginate(func(cursors []string) (nextCursors []string, _ error) {
		return s.exportPullRequestsPageFrom(sessionID, cursors)
	})
}

func (s *Integration) exportPullRequestsPageFrom(sessionID string, cursors []string) (nextCursor []string, _ error) {
	refType := "sourcecode.pull_request"

	query := `
	query { 
		viewer { 
			organization(login:"pinpt"){
				repositories(first: ` + makeAfterParam(cursors, 0) + `) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
					}
					nodes {
						pullRequests(first:` + makeAfterParam(cursors, 0) + `) {
							totalCount
							pageInfo {
								hasNextPage
								endCursor
							}		
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
						TotalCount int `json:"totalCount"`
						PageInfo   struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []struct {
							PullRequests struct {
								TotalCount int `json:"totalCount"`
								PageInfo   struct {
									HasNextPage bool   `json:"hasNextPage"`
									EndCursor   string `json:"endCursor"`
								} `json:"pageInfo"`
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
		return nil, err
	}

	batch := []rpcdef.ExportObj{}

	c := 0
	repos := res.Data.Viewer.Organization.Repositories
	repoNodes := repos.Nodes
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

	return nil, nil
}
