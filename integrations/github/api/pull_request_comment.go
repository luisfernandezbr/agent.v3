package api

import (
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/sourcecode"
)

const prCommentGraphqlFields = `
updatedAt
id
url
pullRequest {
	id
}
repository {
	id
}						
bodyHTML
createdAt
author ` + userFields + `
`

type prCommentGraphql struct {
	UpdatedAt   time.Time `json:"updatedAt"`
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	PullRequest struct {
		ID string `json:"id"`
	} `json:"pullRequest"`
	Repository struct {
		ID string `json:"id"`
	} `json:"repository"`
	BodyHTML  string    `json:"bodyHTML"`
	CreatedAt time.Time `json:"createdAt"`
	Author    User      `json:"author"`
}

func prComment(qc QueryContext, data prCommentGraphql) (res *sourcecode.PullRequestComment, rerr error) {
	item := &sourcecode.PullRequestComment{}
	item.CustomerID = qc.CustomerID
	item.RefType = "github"
	item.RefID = data.ID
	item.URL = data.URL
	date.ConvertToModel(data.UpdatedAt, &item.UpdatedDate)
	item.RepoID = qc.RepoID(data.Repository.ID)
	item.PullRequestID = qc.PullRequestID(item.RepoID, data.PullRequest.ID)
	item.Body = `<div class="source-github">` + data.BodyHTML + `</div>`
	date.ConvertToModel(data.CreatedAt, &item.CreatedDate)

	{
		var err error
		item.UserRefID, err = qc.ExportUserUsingFullDetails(qc.Logger, data.Author)
		if err != nil {
			qc.Logger.Error("could not resolve pr comment author", "login", data.Author.Login, "comment_url", data.URL)
		}
	}

	return item, nil
}

func PullRequestComment(qc QueryContext, commentNodeID string) (res *sourcecode.PullRequestComment, rerr error) {
	qc.Logger.Debug("pull_request_comment request", "comment_node_id", commentNodeID)
	res = &sourcecode.PullRequestComment{}

	query := `
	query {
		node (id: "` + commentNodeID + `") {
			... on IssueComment {
` + prCommentGraphqlFields + `
			}
		}
	}
	`

	var requestRes struct {
		Data struct {
			Node prCommentGraphql `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, nil, &requestRes)
	if err != nil {
		rerr = err
		return
	}

	return prComment(qc, requestRes.Data.Node)
}

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
` + prCommentGraphqlFields + `
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
					TotalCount int                `json:"totalCount"`
					PageInfo   PageInfo           `json:"pageInfo"`
					Nodes      []prCommentGraphql `json:"nodes"`
				} `json:"comments"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, nil, &requestRes)
	if err != nil {
		rerr = err
		return
	}

	nodesContainer := requestRes.Data.Node.Comments
	nodes := nodesContainer.Nodes
	//qc.Logger.Info("got comments", "n", len(nodes))
	for _, data := range nodes {
		item, err := prComment(qc, data)
		if err != nil {
			rerr = err
			return
		}
		res = append(res, item)
	}

	return nodesContainer.PageInfo, res, nodesContainer.TotalCount, nil
}
