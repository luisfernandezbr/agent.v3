package api

import (
	"fmt"
	"net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/commonrepo"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func ApprovedDate(qc QueryContext, repoID string, prIID string, username string) (unixDate int64, err error) {

	qc.Logger.Debug("approval date", "repo", repoID, "prIID", prIID)

	objectPath := pstrings.JoinURL("projects", repoID, "merge_requests", prIID, "discussions")

	var discussions []struct {
		Notes []struct {
			Body      string    `json:"body"`
			CreatedAt time.Time `json:"created_at"`
			Author    struct {
				Username string `json:"username"`
			} `json:"author"`
		} `json:"notes"`
	}

	_, err = qc.Request(objectPath, nil, &discussions)
	if err != nil {
		return
	}

	var date time.Time

	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			if note.Body == "approved this merge request" && note.Author.Username == username && note.CreatedAt.After(date) {
				date = note.CreatedAt
			}
		}
	}

	unixDate = date.Unix()

	return
}

func PullRequestReviewsPage(
	qc QueryContext,
	repo commonrepo.Repo,
	pr PullRequest,
	params url.Values) (pi PageInfo, res []*sourcecode.PullRequestReview, err error) {

	qc.Logger.Debug("pull request reviews", "repo", repo.NameWithOwner, "prID", pr.ID, "prIID", pr.IID)

	objectPath := pstrings.JoinURL("projects", repo.ID, "merge_requests", pr.IID, "approvals")

	var rreview struct {
		ID         int64 `json:"id"`
		ApprovedBy []struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"approved_by"`
		SuggestedApprovers []struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"suggested_approvers"`
		Approvers []struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"approvers"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	pi, err = qc.Request(objectPath, params, &rreview)
	if err != nil {
		return
	}

	for _, a := range rreview.ApprovedBy {
		item := &sourcecode.PullRequestReview{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = fmt.Sprint(rreview.ID)
		item.UpdatedAt, err = ApprovedDate(qc, repo.ID, pr.IID, a.User.Username)
		if err != nil {
			return
		}
		item.RepoID = ids.RepoID(repo.ID, qc)
		item.PullRequestID = ids.PullRequestID(item.RepoID, pr.ID, qc)
		item.State = sourcecode.PullRequestReviewStateApproved

		date.ConvertToModel(rreview.CreatedAt, &item.CreatedDate)

		item.UserRefID = a.User.Username

		res = append(res, item)
	}

	for _, a := range rreview.SuggestedApprovers {
		item := &sourcecode.PullRequestReview{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = fmt.Sprint(rreview.ID)
		item.UpdatedAt = rreview.UpdatedAt.Unix()
		item.RepoID = ids.RepoID(repo.ID, qc)
		item.PullRequestID = ids.PullRequestID(item.RepoID, pr.ID, qc)
		item.State = sourcecode.PullRequestReviewStatePending

		date.ConvertToModel(rreview.CreatedAt, &item.CreatedDate)

		item.UserRefID = a.User.Username

		res = append(res, item)
	}

	return
}
