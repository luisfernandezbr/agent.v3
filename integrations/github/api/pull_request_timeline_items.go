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
	Author    User      `json:"author"`
}

type prReviewRequestChange struct {
	ID                string    `json:"id"`
	CreatedAt         time.Time `json:"createdAt"`
	RequestedReviewer User      `json:"requestedReviewer"`
}

type prAssigneeChange struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Assignee  User      `json:"assignee"`
}

const userFieldsNoBot = `{
	__typename
	... on User {
		id
		name
		avatarUrl
		login
		url		
	}
}`

func GetPullRequestReviewTimelineQuery(pullRequestRefID, queryParams string, assigneeAvailability bool) string {

	var userAssignatedEvent string
	if assigneeAvailability {
		userAssignatedEvent = "assignee " + userFields
	} else {
		userAssignatedEvent = "user " + userFields2
	}

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
							author ` + userFields + `
						}
						... on ReviewRequestedEvent {
							__typename
							id
							createdAt
							requestedReviewer ` + userFieldsNoBot + `
						}
						... on ReviewRequestRemovedEvent {
							__typename
							id
							createdAt
							requestedReviewer ` + userFieldsNoBot + `
						}
						... on AssignedEvent {
							__typename
							id
							createdAt
							` + userAssignatedEvent + `
						}
						... on UnassignedEvent {
							__typename
							id
							createdAt
							` + userAssignatedEvent + `
						}
					}
				}
			}
		}
	}
	`

	return query
}

func PullRequestReviewTimelineItemsPage(
	qc QueryContext,
	repo Repo,
	pullRequestRefID string,
	queryParams string,
	assigneeAvailability bool) (pi PageInfo, res []*sourcecode.PullRequestReview, totalCount int, rerr error) {

	if pullRequestRefID == "" {
		panic("missing pr id")
	}

	logger := qc.Logger.With("pr", pullRequestRefID, "repo", repo.NameWithOwner)

	queryParams += " itemTypes:[PULL_REQUEST_REVIEW,REVIEW_REQUESTED_EVENT,REVIEW_REQUEST_REMOVED_EVENT,ASSIGNED_EVENT,UNASSIGNED_EVENT]"

	logger.Debug("pull_request_timeline_items request", "q", queryParams)

	query := GetPullRequestReviewTimelineQuery(pullRequestRefID, queryParams, assigneeAvailability)

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

	err := qc.Request(query, nil, &requestRes)
	if err != nil {
		rerr = err
		return
	}

	nodesContainer := requestRes.Data.Node.Reviews
	nodes := nodesContainer.Nodes
	for _, m := range nodes {
		typename, _ := m["__typename"].(string)

		item := &sourcecode.PullRequestReview{}
		item.CustomerID = qc.CustomerID
		item.RefType = "github"
		item.RepoID = qc.RepoID(repo.ID)
		item.PullRequestID = qc.PullRequestID(item.RepoID, pullRequestRefID)

		setCommonFields := func(refID string, createdAt time.Time, user User) {
			item.RefID = refID
			date.ConvertToModel(createdAt, &item.CreatedDate)
			{
				var err error
				item.UserRefID, err = qc.ExportUserUsingFullDetails(qc.Logger, user)
				if err != nil {
					qc.Logger.Error("could not resolve pr review author", "login", user.Login)
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
			setCommonFields(data.ID, data.CreatedAt, data.Author)
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
			setCommonFields(data.ID, data.CreatedAt, data.RequestedReviewer)
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
			setCommonFields(data.ID, data.CreatedAt, data.Assignee)
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
