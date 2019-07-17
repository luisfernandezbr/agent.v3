package api

import (
	"time"

	"github.com/pinpt/go-datamodel/sourcecode"
)

func PullRequestCommentsPage(
	qc QueryContext,
	pullRequestRefID string,
	queryParams string) (pi PageInfo, res []sourcecode.PullRequestComment, _ error) {

	if pullRequestRefID == "" {
		panic("mussing pr id")
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
		return pi, res, err
	}

	nodesContainer := requestRes.Data.Node.Comments
	nodes := nodesContainer.Nodes
	//qc.Logger.Info("got comments", "n", len(nodes))
	for _, data := range nodes {
		item := sourcecode.PullRequestComment{}
		item.CustomerID = qc.CustomerID
		item.RefType = "sourcecode.pull_request_comment"
		item.RefID = data.ID
		item.Updated = TimePullRequestCommentUpdated(data.UpdatedAt)

		item.RepoID = qc.RepoID(data.Repository.ID)
		item.PullRequestID = qc.PullRequestID(data.PullRequest.ID)
		item.Body = data.BodyText
		item.Created = TimePullRequestCommentCreated(data.CreatedAt)

		item.UserRefID, err = qc.UserLoginToRefID(data.Author.Login)
		if err != nil {
			panic(err)
		}
		res = append(res, item)
	}

	return nodesContainer.PageInfo, res, nil
}
