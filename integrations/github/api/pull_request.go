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
}

func PullRequestsPage(
	qc QueryContext,
	repoRefID string,
	queryParams string, stopOnUpdatedAt time.Time) (pi PageInfo, res []PullRequest, totalCount int, rerr error) {

	qc.Logger.Debug("pull_request request", "repo", repoRefID, "q", queryParams)

	//useClosedEvents := !qc.IsEnterprise()
	// TODO: if we want to support GitHub Enterprise 2.15.9 we need to get this info in another way. Also need to be able to check the api version.
	// For not we only support Github Enterprise 2.16 and higher.
	useClosedEvents := true

	closedEventsQ := ``

	if useClosedEvents {
		closedEventsQ = `
		# fetch the user who closed the pull request
		# this is only relevant when the state = CLOSED
		closedEvents: timelineItems (last:1 itemTypes:CLOSED_EVENT){
			nodes {
				... on ClosedEvent {
					actor {
						login
					}
				}
			}
		}
		`
	}

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
						updatedAt
						id
						number
						repository {
							id
							nameWithOwner
						}
						headRefName
						title
						bodyText
						url
						createdAt
						mergedAt
						closedAt
						# OPEN, CLOSED or MERGED
						state
						author { login }
						mergedBy { login }
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
						` + closedEventsQ + `
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
					TotalCount int      `json:"totalCount"`
					PageInfo   PageInfo `json:"pageInfo"`
					Nodes      []struct {
						ID         string `json:"id"`
						Repository struct {
							ID   string `json:"id"`
							Name string `json:"nameWithOwner"`
						}
						Number      int       `json:"number"`
						HeadRefName string    `json:"headRefName"`
						Title       string    `json:"title"`
						BodyText    string    `json:"bodyText"`
						URL         string    `json:"url"`
						CreatedAt   time.Time `json:"createdAt"`
						MergedAt    time.Time `json:"mergedAt"`
						ClosedAt    time.Time `json:"closedAt"`
						UpdatedAt   time.Time `json:"updatedAt"`
						State       string    `json:"state"`
						Author      struct {
							Login string `json:"login"`
						} `json:"author"`
						MergedBy struct {
							Login string `json:"login"`
						} `json:"mergedBy"`
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
								Actor struct {
									Login string `json:"login"`
								} `json:"actor"`
							} `json:"nodes"`
						} `json:"closedEvents"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, &requestRes)
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
		pr := &sourcecode.PullRequest{}
		pr.CustomerID = qc.CustomerID
		pr.RefType = "github"
		pr.RefID = data.ID
		pr.RepoID = qc.RepoID(repoRefID)
		pr.BranchName = data.HeadRefName
		pr.Title = data.Title
		pr.Description = data.BodyText
		pr.URL = data.URL
		pr.Identifier = fmt.Sprintf("%s#%d", data.Repository.Name, data.Number) // such as pinpt/datamodel#123 which is the display format GH uses
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

		{
			login := data.Author.Login
			pr.CreatedByRefID, err = qc.UserLoginToRefID(data.Author.Login)
			if err != nil {
				qc.Logger.Error("could not resolve pr created by user", "login", login, "pr_url", data.URL)
			}
		}

		if useClosedEvents && data.State == "CLOSED" {
			events := data.ClosedEvents.Nodes
			if len(events) != 0 {
				login := events[0].Actor.Login
				if login == "" {
					qc.Logger.Error("pull request: empty login for closed by author field", "pr_url", data.URL)
				} else {
					pr.ClosedByRefID, err = qc.UserLoginToRefID(login)
					if err != nil {
						qc.Logger.Error("could not resolve closed by user when processing pr", "login", login, "pr_url", data.URL)
					}
				}
			} else {
				qc.Logger.Error("pr status is CLOSED, but no closed events found, can't set ClosedBy")
			}
		}

		if data.State == "MERGED" {
			pr.MergeSha = data.MergeCommit.OID
			pr.MergeCommitID = ids.CodeCommit(qc.CustomerID, qc.RefType, pr.RepoID, pr.MergeSha)
			login := data.MergedBy.Login
			if login == "" {
				qc.Logger.Error("pull request: empty login for mergedBy field", "pr_url", data.URL)
			} else {
				pr.MergedByRefID, err = qc.UserLoginToRefID(login)
				if err != nil {
					qc.Logger.Error("could not resolve merged by user when processing pr", "login", login, "pr_url", data.URL)
				}
			}
		}

		pr2 := PullRequest{}
		pr2.PullRequest = pr
		pr2.HasComments = data.Comments.TotalCount != 0
		pr2.HasReviews = data.Reviews.TotalCount != 0
		if len(data.LastCommits.Nodes) != 0 {
			pr2.LastCommitSHA = data.LastCommits.Nodes[0].Commit.OID
		}
		res = append(res, pr2)
	}

	return pullRequests.PageInfo, res, pullRequests.TotalCount, nil
}
