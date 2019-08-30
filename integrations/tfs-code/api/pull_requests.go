package api

import (
	"fmt"
	purl "net/url"
	"time"

	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type pullRequestResponse struct {
	ID        int64 `json:"pullRequestId"`
	CreatedBy struct {
		DisplayName string `json:"displayName"`
		ID          string `json:"id"`
	} `json:"createdBy"`
	CreationDate string `json:"creationDate"`
	ClosedDate   string `json:"closedDate"`
	Description  string `json:"description"`
	MergeStatus  string `json:"mergeStatus"`
	Repository   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	Reviewers []struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		UniqueName  string `json:"uniqueName"`
		Vote        int64  `json:"vote"`
	} `json:"reviewers"`
	Status       string `json:"status"`
	SourceBranch string `json:"sourceRefName"`
	TargetBranch string `json:"targetRefName"`
	Title        string `json:"title"`
	URL          string `json:"url"`
}

// FetchPullRequests calls the pull request api returns a list of sourcecode.PullRequest and sourcecode.PullRequestReview
func (a *TFSAPI) FetchPullRequests(repoid string) (prs []*sourcecode.PullRequest, prrs []*sourcecode.PullRequestReview, err error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests`, purl.PathEscape(repoid))
	var res []pullRequestResponse
	if err = a.doRequest(url, params{
		"searchCriteria.status": "all",
	}, time.Time{}, &res); err != nil {
		fmt.Println(err)
		return
	}
	for _, p := range res {
		pr := &sourcecode.PullRequest{
			BranchID:       a.BranchID(repoid, p.SourceBranch),
			CreatedByRefID: p.CreatedBy.ID,
			Description:    p.Description,
			RefID:          fmt.Sprintf("%d", p.ID),
			RefType:        a.reftype,
			CustomerID:     a.customerid,
			RepoID:         a.RepoID(p.Repository.ID),
			Title:          p.Title,
			URL:            p.URL,
		}
		switch p.Status {
		case "completed":
			pr.Status = sourcecode.PullRequestStatusMerged
		case "active":
			pr.Status = sourcecode.PullRequestStatusOpen
		case "abandoned":
			pr.Status = sourcecode.PullRequestStatusClosed
		}
		for i, r := range p.Reviewers {

			var state sourcecode.PullRequestReviewState
			switch r.Vote {
			case -10:
				pr.ClosedByRefID = r.ID
				state = sourcecode.PullRequestReviewStateDismissed
			case -5:
				state = sourcecode.PullRequestReviewStateChangesRequested
			case 0:
				state = sourcecode.PullRequestReviewStatePending
			case 5:
				state = sourcecode.PullRequestReviewStateCommented
			case 10:
				pr.MergedByRefID = r.ID
				state = sourcecode.PullRequestReviewStateApproved
			}
			prrs = append(prrs, &sourcecode.PullRequestReview{
				RefID:         hash.Values(i, r.ID), // this id is from the person, there are no "ids" for reviews
				RefType:       a.reftype,
				CustomerID:    a.customerid,
				RepoID:        a.RepoID(repoid),
				State:         state,
				UserRefID:     r.UniqueName,
				PullRequestID: a.PullRequestID(fmt.Sprintf("%d", p.ID)),
			})
		}
		if p.ClosedDate != "" {
			d, e := datetime.NewDate(p.ClosedDate)
			if e != nil {
				a.logger.Warn("error converting date in tfs-code FetchPullRequests 1")
			} else {

			}
			pr.ClosedDate = sourcecode.PullRequestClosedDate(*d)
		}
		if p.CreationDate != "" {
			d, e := datetime.NewDate(p.CreationDate)
			if e != nil {
				a.logger.Warn("error converting date in tfs-code FetchPullRequests 1")
			} else {

			}
			pr.CreatedDate = sourcecode.PullRequestCreatedDate(*d)

		}
		prs = append(prs, pr)
	}
	return
}
