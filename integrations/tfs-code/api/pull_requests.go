package api

import (
	"fmt"
	purl "net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
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
	CreationDate          string `json:"creationDate"`
	ClosedDate            string `json:"closedDate"`
	Description           string `json:"description"`
	MergeStatus           string `json:"mergeStatus"`
	LastMergeSourceCommit struct {
		CommidID string `json:"commitId"`
	} `json:"lastMergeSourceCommit"`
	LastMergeCommit struct {
		CommidID string `json:"commitId"`
	} `json:"lastMergeCommit"`
	Repository struct {
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

type pullRequestCommitsResponse struct {
	CommitID string `json:"commitId"`
}

func (a *TFSAPI) fetchPullRequestCommits(repoid string, prid int64) (shas []string, err error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/commits`, purl.PathEscape(repoid), prid)
	var res []pullRequestCommitsResponse
	err = a.doRequest(url, nil, time.Now(), &res)
	for _, c := range res {
		shas = append(shas, c.CommitID)
	}
	return
}

// FetchPullRequests calls the pull request api returns a list of sourcecode.PullRequest and sourcecode.PullRequestReview
func (a *TFSAPI) FetchPullRequests(repoid string) (prs []*sourcecode.PullRequest, prrs []*sourcecode.PullRequestReview, err error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests`, purl.PathEscape(repoid))
	var res []pullRequestResponse
	if err = a.doRequest(url, params{"status": "all"}, time.Time{}, &res); err != nil {
		return
	}
	for _, p := range res {
		commits, err := a.fetchPullRequestCommits(repoid, p.ID)
		if err != nil {
			return nil, nil, err
		}
		pr := &sourcecode.PullRequest{
			CreatedByRefID: p.CreatedBy.ID,
			Description:    p.Description,
			BranchName:     p.SourceBranch,
			RefID:          fmt.Sprintf("%d", p.ID),
			RefType:        a.reftype,
			CustomerID:     a.customerid,
			RepoID:         a.RepoID(p.Repository.ID),
			Title:          p.Title,
			URL:            p.URL,
			CommitShas:     commits,
		}
		if len(commits) != 0 {
			pr.BranchID = a.BranchID(repoid, p.SourceBranch, commits[0])
		}
		pr.CommitIds = ids.CodeCommits(a.customerid, a.reftype, pr.RepoID, commits)

		switch p.Status {
		case "completed":
			pr.Status = sourcecode.PullRequestStatusMerged
			pr.MergeSha = p.LastMergeCommit.CommidID
			pr.MergeCommitID = ids.CodeCommit(a.customerid, a.reftype, pr.RepoID, pr.MergeSha)
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
			refid := hash.Values(i, r.ID)
			prrs = append(prrs, &sourcecode.PullRequestReview{
				RefID:         refid, // this id is from the person, there are no "ids" for reviews
				RefType:       a.reftype,
				CustomerID:    a.customerid,
				RepoID:        a.RepoID(repoid),
				State:         state,
				UserRefID:     r.ID,
				PullRequestID: a.PullRequestID(fmt.Sprintf("%d", p.ID), refid),
			})
		}
		if p.ClosedDate != "" {
			if d, err := datetime.ISODateToTime(p.ClosedDate); err != nil {
				a.logger.Error("error converting date in tfs-code FetchPullRequests 1")
			} else {
				date.ConvertToModel(d, &pr.ClosedDate)
			}
		}
		if p.CreationDate != "" {
			if d, err := datetime.ISODateToTime(p.CreationDate); err != nil {
				a.logger.Error("error converting date in tfs-code FetchPullRequests 1")
			} else {
				date.ConvertToModel(d, &pr.CreatedDate)
			}
		}
		prs = append(prs, pr)
	}

	return
}
