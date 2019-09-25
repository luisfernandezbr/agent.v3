package api

import (
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func PullRequestCommentsPage(
	qc QueryContext,
	pullRequestRefID string,
	queryParams string) (pi PageInfo, res []*sourcecode.PullRequestComment, totalCount int, rerr error) {

	if pullRequestRefID == "" {
		panic("missing pr id")
	}

	qc.Logger.Debug("pull_request_comments request", "pr", pullRequestRefID, "q", queryParams)

	query := `
	query {
		node (id: "` + pullRequestRefID + `") {
			... on PullRequest {
				comments(` + queryParams + `) {
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
						url
						pullRequest {
							id
						}
						repository {
							id
						}						
						bodyText
						createdAt
						author {
							login
						}
					}
				}
			}
		}
	}
	`

	var requestRes struct {
		Data struct {
			Node struct {
				Comments struct {
					TotalCount int      `json:"totalCount"`
					PageInfo   PageInfo `json:"pageInfo"`
					Nodes      []struct {
						UpdatedAt   time.Time `json:"updatedAt"`
						ID          string    `json:"id"`
						URL         string    `json:"url"`
						PullRequest struct {
							ID string `json:"id"`
						} `json:"pullRequest"`
						Repository struct {
							ID string `json:"id"`
						} `json:"repository"`
						//Body string `json:body`
						BodyText  string    `json:"bodyText"`
						CreatedAt time.Time `json:"createdAt"`
						Author    struct {
							Login string `json:"login"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"comments"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, &requestRes)
	if err != nil {
		rerr = err
		return
	}

	nodesContainer := requestRes.Data.Node.Comments
	nodes := nodesContainer.Nodes
	//qc.Logger.Info("got comments", "n", len(nodes))
	for _, data := range nodes {
		item := &sourcecode.PullRequestComment{}
		item.CustomerID = qc.CustomerID
		item.RefType = "github"
		item.RefID = data.ID
		item.URL = data.URL
		date.ConvertToModel(data.UpdatedAt, &item.UpdatedDate)
		item.RepoID = qc.RepoID(data.Repository.ID)
		item.PullRequestID = qc.PullRequestID(item.RepoID, data.PullRequest.ID)
		item.Body = data.BodyText
		date.ConvertToModel(data.CreatedAt, &item.CreatedDate)

		{
			login := data.Author.Login
			item.UserRefID, err = qc.UserLoginToRefID(login)
			if err != nil {
				qc.Logger.Error("could not resolve pr comment author", "login", login, "comment_url", data.URL)
			}
		}

		res = append(res, item)
	}

	return nodesContainer.PageInfo, res, nodesContainer.TotalCount, nil
}
