package api

import (
	"time"

	"github.com/pinpt/go-datamodel/sourcecode"
)

type PullRequest struct {
	sourcecode.PullRequest
	HasComments bool
	HasReviews  bool
}

func PullRequestsPage(
	qc QueryContext,
	repoRefID string,
	queryParams string, stopOnUpdatedAt time.Time) (pi PageInfo, res []PullRequest, _ error) {

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
						updatedAt
						id
						repository { id }
						title
						bodyText
						url
						createdAt
						mergedAt
						closedAt
						# OPEN, CLOSED or MERGED
						state
						author { login }
						comments {
							totalCount
						}
						reviews {
							totalCount
						}
						# fetch the user who closed the issues
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
							ID string `json:"id"`
						}
						Title    string `json:"title"`
						BodyText string `json:"bodyText"`

						URL       string    `json:"url"`
						CreatedAt time.Time `json:"createdAt"`
						MergedAt  time.Time `json:"mergedAt"`
						ClosedAt  time.Time `json:"closedAt"`
						UpdatedAt time.Time `json:"updatedAt"`
						State     string    `json:"state"`
						Author    struct {
							Login string `json:"login"`
						}
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

	// TODO: why don't we have merged_by?

	err := qc.Request(query, &requestRes)
	if err != nil {
		return pi, res, err
	}

	//s.logger.Info(fmt.Sprintf("%+v", res))

	pullRequests := requestRes.Data.Node.PullRequests
	pullRequestNodes := pullRequests.Nodes

	for _, data := range pullRequestNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			return PageInfo{}, res, nil
		}
		pr := sourcecode.PullRequest{}
		pr.CustomerID = qc.CustomerID
		pr.RefType = "sourcecode.pull_request"
		pr.RefID = data.ID
		pr.RepoID = qc.RepoID(repoRefID)
		pr.Title = data.Title
		pr.Description = data.BodyText
		pr.URL = data.URL
		pr.CreatedAt = data.CreatedAt.Unix()
		pr.MergedAt = data.MergedAt.Unix()

		pr.ClosedAt = data.ClosedAt.Unix()
		pr.UpdatedAt = data.UpdatedAt.Unix()
		validStatus := []string{"OPEN", "CLOSED", "MERGED"}
		if !strInArr(data.State, validStatus) {
			panic("unknown state: " + data.State)
		}
		pr.Status = data.State
		pr.UserRefID, err = qc.UserLoginToRefID(data.Author.Login)
		if err != nil {
			panic(err)
		}

		if data.State == "CLOSED" {
			events := data.ClosedEvents.Nodes
			if len(events) != 0 {
				login := events[0].Actor.Login
				if login == "" {
					qc.Logger.Error("empty login")
				}
				pr.ClosedByRefID, err = qc.UserLoginToRefID(login)
				if err != nil {
					panic(err)
				}
			} else {
				qc.Logger.Error("pr status is CLOSED, but no closed events found, can't set ClosedBy")
			}
		}

		pr2 := PullRequest{}
		pr2.PullRequest = pr
		pr2.HasComments = data.Comments.TotalCount != 0
		pr2.HasReviews = data.Reviews.TotalCount != 0
		res = append(res, pr2)
	}

	return pullRequests.PageInfo, res, nil
}
