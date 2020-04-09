package api

import (
	"time"

	"github.com/pinpt/agent/pkg/ids"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func PullRequestCommitsPage(
	qc QueryContext,
	pullRequestRefID string,
	queryParams string) (pi PageInfo, res []*sourcecode.PullRequestCommit, rerr error) {

	if pullRequestRefID == "" {
		panic("missing pr id")
	}

	qc.Logger.Debug("pull_request_commits request", "pr", pullRequestRefID, "q", queryParams)

	query := `
	query {
		node (id: "` + pullRequestRefID + `") {
			... on PullRequest {
				repository {
					id 
				}
				commits(` + queryParams + `) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
						hasPreviousPage
						startCursor
					}
					nodes {
						commit {
							oid
							message
							url
							additions
							deletions
							author {
								email
							}
							committer {
								email
							}
							authoredDate
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
				Repository struct {
					ID string `json:"id"`
				} `json:"repository"`
				Commits struct {
					TotalCount int      `json:"totalCount"`
					PageInfo   PageInfo `json:"pageInfo"`
					Nodes      []struct {
						Commit struct {
							OID       string `json:"oid"`
							Message   string `json:"message"`
							URL       string `json:"url"`
							Additions int    `json:"additions"`
							Deletions int    `json:"deletions"`
							Author    struct {
								Email string `json:"email"`
							} `json:"author"`
							Committer struct {
								Email string `json:"email"`
							} `json:"committer"`
							AuthoredData time.Time `json:"authoredDate"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, nil, &requestRes)
	if err != nil {
		rerr = err
		return
	}

	nodesContainer := requestRes.Data.Node.Commits
	nodes := nodesContainer.Nodes

	repoID := qc.RepoID(requestRes.Data.Node.Repository.ID)

	for _, node := range nodes {
		data := node.Commit

		item := &sourcecode.PullRequestCommit{}
		item.CustomerID = qc.CustomerID
		item.RefType = "github"
		item.RefID = data.OID

		item.RepoID = repoID
		item.PullRequestID = qc.PullRequestID(item.RepoID, pullRequestRefID)
		item.Sha = data.OID
		item.Message = data.Message
		item.URL = data.URL
		date.ConvertToModel(data.AuthoredData, &item.CreatedDate)

		item.Additions = int64(data.Additions)
		item.Deletions = int64(data.Deletions)
		// not setting branch, too difficult
		item.AuthorRefID = ids.CodeCommitEmail(qc.CustomerID, data.Author.Email)
		item.CommitterRefID = ids.CodeCommitEmail(qc.CustomerID, data.Committer.Email)

		res = append(res, item)
	}

	return nodesContainer.PageInfo, res, nil
}
