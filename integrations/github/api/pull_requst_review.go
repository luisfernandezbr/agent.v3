package api

import (
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func PullRequestReviewsPage(
	qc QueryContext,
	pullRequestRefID string,
	queryParams string) (pi PageInfo, res []*sourcecode.PullRequestReview, _ error) {

	if pullRequestRefID == "" {
		panic("mussing pr id")
	}

	qc.Logger.Debug("pull_request_reviews request", "pr", pullRequestRefID, "q", queryParams)

	query := `
	query {
		node (id: "` + pullRequestRefID + `") {
			... on PullRequest {
				reviews(` + queryParams + `) {
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
						state
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
				Reviews struct {
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
						// PENDING,COMMENTED,APPROVED,CHANGES_REQUESTED or DISMISSED
						State     string    `json:"state"`
						CreatedAt time.Time `json:"createdAt"`
						Author    struct {
							Login string `json:"login"`
						}
					} `json:"nodes"`
				} `json:"reviews"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, &requestRes)
	if err != nil {
		return pi, res, err
	}

	//qc.Logger.Info(fmt.Sprintf("%+v", res))

	nodesContainer := requestRes.Data.Node.Reviews
	nodes := nodesContainer.Nodes
	//qc.Logger.Info("got comments", "n", len(nodes))
	for _, data := range nodes {
		item := &sourcecode.PullRequestReview{}
		item.CustomerID = qc.CustomerID
		item.RefType = "sourcecode.pull_request_review"
		item.RefID = data.ID
		//item.UpdatedAt = data.UpdatedAt.Unix()
		item.RepoID = qc.RepoID(data.Repository.ID)
		item.PullRequestID = qc.PullRequestID(data.PullRequest.ID)
		item.State = data.State

		date.ConvertToModel(data.CreatedAt, &item.CreatedDate)

		item.UserRefID, err = qc.UserLoginToRefID(data.Author.Login)
		if err != nil {
			panic(err)
		}
		res = append(res, item)
	}

	return nodesContainer.PageInfo, res, nil
}
