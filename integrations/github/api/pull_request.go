package api

import (
	"fmt"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type PullRequest struct {
	*sourcecode.PullRequest
	HasComments   bool
	HasReviews    bool
	LastCommitSHA string
	Repo          Repo
}

const pullRequestFieldsGraphql = `
updatedAt
id
number
repository {
	id
	nameWithOwner
}
headRefName
title
bodyHTML
url
createdAt
mergedAt
closedAt
# OPEN, CLOSED or MERGED
state
draft: isDraft
locked
author ` + userFields + `
mergedBy ` + userFields + `
mergeCommit { oid }
commits(last: 1) {
	nodes {
		commit {
			oid
		}
	}
}
comments {
	totalCount
}
reviews {
	totalCount
}
# fetch the user who closed the pull request
# this is only relevant when the state = CLOSED
closedEvents: timelineItems (last:1 itemTypes:CLOSED_EVENT){
	nodes {
		... on ClosedEvent {
			actor ` + userFields + `
		}
	}
}
`

type pullRequestGraphql struct {
	ID         string `json:"id"`
	Repository struct {
		ID            string `json:"id"`
		NameWithOwner string `json:"nameWithOwner"`
	}
	Number      int       `json:"number"`
	HeadRefName string    `json:"headRefName"`
	Title       string    `json:"title"`
	BodyHTML    string    `json:"bodyHTML"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"createdAt"`
	MergedAt    time.Time `json:"mergedAt"`
	ClosedAt    time.Time `json:"closedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	State       string    `json:"state"`
	Draft       bool      `json:"draft"`
	Locked      bool      `json:"locked"`
	Author      User      `json:"author"`
	MergedBy    User      `json:"mergedBy"`
	MergeCommit struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
	LastCommits struct {
		Nodes []struct {
			Commit struct {
				OID string `json:"oid"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Comments struct {
		TotalCount int `json:"totalCount"`
	} `json:"comments"`
	Reviews struct {
		TotalCount int `json:"totalCount"`
	} `json:"reviews"`
	ClosedEvents struct {
		Nodes []struct {
			Actor User `json:"actor"`
		} `json:"nodes"`
	} `json:"closedEvents"`
}

func convertPullRequest(qc QueryContext, data pullRequestGraphql) PullRequest {
	pr := &sourcecode.PullRequest{}
	pr.CustomerID = qc.CustomerID
	pr.RefType = "github"
	pr.RefID = data.ID
	pr.RepoID = qc.RepoID(data.Repository.ID)

	pr.BranchName = data.HeadRefName
	pr.Title = data.Title
	pr.Description = `<div class="source-github">` + data.BodyHTML + `</div>`
	pr.URL = data.URL
	pr.Identifier = fmt.Sprintf("%s#%d", data.Repository.NameWithOwner, data.Number) // such as pinpt/datamodel#123 which is the display format GH uses
	date.ConvertToModel(data.CreatedAt, &pr.CreatedDate)
	date.ConvertToModel(data.MergedAt, &pr.MergedDate)
	date.ConvertToModel(data.ClosedAt, &pr.ClosedDate)
	date.ConvertToModel(data.UpdatedAt, &pr.UpdatedDate)
	switch data.State {
	case "OPEN":
		pr.Status = sourcecode.PullRequestStatusOpen
	case "CLOSED":
		pr.Status = sourcecode.PullRequestStatusClosed
	case "MERGED":
		pr.Status = sourcecode.PullRequestStatusMerged
	default:
		qc.Logger.Error("could not process pr state, state is unknown", "state", data.State, "pr_url", data.URL)
	}

	if data.Locked {
		pr.Status = sourcecode.PullRequestStatusLocked
	}

	pr.Draft = data.Draft

	if data.State == "MERGED" {
		pr.MergeSha = data.MergeCommit.OID
		pr.MergeCommitID = ids.CodeCommit(qc.CustomerID, qc.RefType, pr.RepoID, pr.MergeSha)
	}

	// only set those fields in exports and webhooks, not in mutations (TODO: maybe change this)
	if qc.ExportUserUsingFullDetails != nil {

		{
			login := data.Author.Login
			var err error
			pr.CreatedByRefID, err = qc.ExportUserUsingFullDetails(qc.Logger, data.Author)
			if err != nil {
				qc.Logger.Error("could not resolve pr created by user", "login", login, "pr_url", data.URL)
			}
		}

		if data.State == "CLOSED" {
			events := data.ClosedEvents.Nodes
			if len(events) != 0 {
				login := events[0].Actor.Login
				if login == "" {
					qc.Logger.Error("pull request: empty login for closed by author field", "pr_url", data.URL)
				} else {
					var err error
					pr.ClosedByRefID, err = qc.ExportUserUsingFullDetails(qc.Logger, events[0].Actor)
					if err != nil {
						qc.Logger.Error("could not resolve closed by user when processing pr", "login", login, "pr_url", data.URL)
					}
				}
			} else {
				qc.Logger.Error("pr status is CLOSED, but no closed events found, can't set ClosedBy")
			}
		}

		if data.State == "MERGED" {
			login := data.MergedBy.Login
			if login == "" {
				qc.Logger.Error("pull request: empty login for mergedBy field", "pr_url", data.URL)
			} else {
				var err error
				pr.MergedByRefID, err = qc.ExportUserUsingFullDetails(qc.Logger, data.MergedBy)
				if err != nil {
					qc.Logger.Error("could not resolve merged by user when processing pr", "login", login, "pr_url", data.URL)
				}
			}
		}
	}

	pr2 := PullRequest{}
	pr2.PullRequest = pr
	pr2.Repo.ID = data.Repository.ID
	pr2.Repo.NameWithOwner = data.Repository.NameWithOwner
	pr2.HasComments = data.Comments.TotalCount != 0
	pr2.HasReviews = data.Reviews.TotalCount != 0
	if len(data.LastCommits.Nodes) != 0 {
		pr2.LastCommitSHA = data.LastCommits.Nodes[0].Commit.OID
	}

	return pr2
}

func PullRequestByID(qc QueryContext, refID string) (_ PullRequest, rerr error) {

	query := `
	query {
		node (id: "` + refID + `") {
			... on PullRequest {
				` + pullRequestFieldsGraphql + `
			}
		}
	}
	`

	var requestRes struct {
		Data struct {
			Node pullRequestGraphql `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, nil, &requestRes)
	if err != nil {
		rerr = err
		return
	}

	return convertPullRequest(qc, requestRes.Data.Node), nil
}

func PullRequestsPage(
	qc QueryContext,
	repoRefID string,
	queryParams string, stopOnUpdatedAt time.Time) (pi PageInfo, res []PullRequest, totalCount int, rerr error) {

	qc.Logger.Debug("pull_request request", "repo", repoRefID, "q", queryParams)

	query := `
	query {
		node (id: "` + repoRefID + `") {
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
						` + pullRequestFieldsGraphql + `
					}
				}
			}
		}
	}
	`

	var requestRes struct {
		Data struct {
			Node struct {
				PullRequests struct {
					TotalCount int                  `json:"totalCount"`
					PageInfo   PageInfo             `json:"pageInfo"`
					Nodes      []pullRequestGraphql `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, nil, &requestRes)
	if err != nil {
		rerr = err
		return
	}

	//s.logger.Info(fmt.Sprintf("%+v", res))

	pullRequests := requestRes.Data.Node.PullRequests
	pullRequestNodes := pullRequests.Nodes

	for _, data := range pullRequestNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			return
		}
		pr2 := convertPullRequest(qc, data)
		res = append(res, pr2)
	}

	return pullRequests.PageInfo, res, pullRequests.TotalCount, nil
}
