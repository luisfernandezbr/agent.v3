package api

import (
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type pullRequestReview struct {
	ID  string `json:"id"`
	URL string `json:"url"`
	// PENDING,COMMENTED,APPROVED,CHANGES_REQUESTED or DISMISSED
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	Author    struct {
		Login string `json:"login"`
	} `json:"author"`
}

type prReviewRequestChange struct {
	ID                string    `json:"id"`
	CreatedAt         time.Time `json:"createdAt"`
	RequestedReviewer struct {
		Login string `json:"login"`
	} `json:"requestedReviewer"`
}

type prAssigneeChange struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Assignee  struct {
		Login string `json:"login"`
	} `json:"assignee"`
}

func PullRequestReviewTimelineItemsPage(
	qc QueryContext,
	repo Repo,
	pullRequestRefID string,
	queryParams string) (pi PageInfo, res []*sourcecode.PullRequestReview, totalCount int, rerr error) {

	if pullRequestRefID == "" {
		panic("missing pr id")
	}

	logger := qc.Logger.With("pr", pullRequestRefID, "repo", repo.NameWithOwner)

	logger.Debug("pull_request_timeline_items request", "q", queryParams)

	query := `
	query {
		node (id: "` + pullRequestRefID + `") {
			... on PullRequest {
				timelineItems(` + queryParams + `) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
						hasPreviousPage
						startCursor
					}
					nodes {
						... on PullRequestReview {
							__typename
							id
							url
							state
							createdAt
							author {
								login
							}
						}
						... on ReviewRequestedEvent {
							__typename
							id
							createdAt
							requestedReviewer {
								... on User {
									login
								}
							}
						}
						... on ReviewRequestRemovedEvent {
							__typename
							id
							createdAt
							requestedReviewer {
								... on User {
									login
								}
							}
						}
						... on AssignedEvent {
							__typename
							id
							createdAt
							assignee {
								... on User {
									login
								}
							}
						}
						... on UnassignedEvent {
							__typename
							id
							createdAt
							assignee {
								... on User {
									login
								}
							}
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
					TotalCount int                      `json:"totalCount"`
					PageInfo   PageInfo                 `json:"pageInfo"`
					Nodes      []map[string]interface{} `json:"nodes"`
				} `json:"timelineItems"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, &requestRes)
	if err != nil {
		rerr = err
		return
	}

	//qc.Logger.Info(fmt.Sprintf("%+v", res))

	nodesContainer := requestRes.Data.Node.Reviews
	nodes := nodesContainer.Nodes
	//qc.Logger.Info("got reviews", "n", len(nodes))
	for _, m := range nodes {
		typename, _ := m["__typename"].(string)

		item := &sourcecode.PullRequestReview{}
		item.CustomerID = qc.CustomerID
		item.RefType = "github"
		item.RepoID = qc.RepoID(repo.ID)
		item.PullRequestID = qc.PullRequestID(item.RepoID, pullRequestRefID)

		setCommonFields := func(refID string, createdAt time.Time, userLogin string) {
			item.RefID = refID
			date.ConvertToModel(createdAt, &item.CreatedDate)
			{
				item.UserRefID, err = qc.UserLoginToRefID(userLogin)
				if err != nil {
					qc.Logger.Error("could not resolve pr review author", "login", userLogin)
				}
			}
		}

		switch typename {
		case "":
			continue
		case "PullRequestReview":
			var data pullRequestReview
			err := structmarshal.MapToStruct(m, &data)
			if err != nil {
				rerr = err
				return
			}
			item.URL = data.URL
			setCommonFields(data.ID, data.CreatedAt, data.Author.Login)
			switch data.State {
			case "PENDING":
				item.State = sourcecode.PullRequestReviewStatePending
			case "COMMENTED":
				item.State = sourcecode.PullRequestReviewStateCommented
			case "APPROVED":
				item.State = sourcecode.PullRequestReviewStateApproved
			case "CHANGES_REQUESTED":
				item.State = sourcecode.PullRequestReviewStateChangesRequested
			case "DISMISSED":
				item.State = sourcecode.PullRequestReviewStateDismissed
			}
		case "ReviewRequestedEvent", "ReviewRequestRemovedEvent":
			var data prReviewRequestChange
			err := structmarshal.MapToStruct(m, &data)
			if err != nil {
				rerr = err
				return
			}
			if data.RequestedReviewer.Login == "" {
				logger.Debug("skipped review request event, since it did not have login for reviewer user (we don't support team reviewers)")
				continue
			}
			setCommonFields(data.ID, data.CreatedAt, data.RequestedReviewer.Login)
			switch typename {
			case "ReviewRequestedEvent":
				item.State = sourcecode.PullRequestReviewStateRequested
			case "ReviewRequestRemovedEvent":
				item.State = sourcecode.PullRequestReviewStateRequestRemoved
			default:
				panic("invalid typename")
			}

		case "AssignedEvent", "UnassignedEvent":
			var data prAssigneeChange
			err := structmarshal.MapToStruct(m, &data)
			if err != nil {
				rerr = err
				return
			}
			if data.Assignee.Login == "" {
				logger.Debug("skipped assigned event, since it did not have login for assigned user (we don't support team assignees)")
				continue
			}
			setCommonFields(data.ID, data.CreatedAt, data.Assignee.Login)
			switch typename {
			case "AssignedEvent":
				item.State = sourcecode.PullRequestReviewStateAssigned
			case "UnassignedEvent":
				item.State = sourcecode.PullRequestReviewStateUnassigned
			default:
				panic("invalid typename")
			}
		}

		res = append(res, item)
	}

	return nodesContainer.PageInfo, res, nodesContainer.TotalCount, nil
}
