package api

import (
	"fmt"
	"net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func PullRequestPage(
	qc QueryContext,
	repoID string,
	repoName string,
	params url.Values,
	stopOnUpdatedAt time.Time) (pi PageInfo, res []sourcecode.PullRequest, err error) {

	qc.Logger.Debug("repo pull requests", "repo", repoName)

	objectPath := pstrings.JoinURL("repositories", repoName, "pullrequests")
	params.Add("state", "MERGED")
	params.Add("state", "SUPERSEDED")
	params.Add("state", "OPEN")
	params.Add("state", "DECLINED")
	params.Set("sort", "-updated_on")

	var rprs []struct {
		ID     int64 `json:"id"`
		Source struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		} `json:"source"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Links       struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
		CreatedOn time.Time `json:"created_on"`
		UpdatedOn time.Time `json:"updated_on"`
		State     string    `json:"state"`
		ClosedBy  struct {
			AccountID string `json:"account_id"`
		} `json:"closed_by"`
		MergeCommit struct {
			Hash string `json:"hash"`
		} `json:"merge_commit"`
		Author struct {
			AccountID string `json:"account_id"`
		} `json:"author"`
	}

	pi, err = qc.Request(objectPath, params, true, &rprs)
	if err != nil {
		return
	}

	for _, rpr := range rprs {
		if rpr.UpdatedOn.Before(stopOnUpdatedAt) {
			return pi, res, nil
		}
		pr := sourcecode.PullRequest{}
		pr.CustomerID = qc.CustomerID
		pr.RefType = qc.RefType
		pr.RefID = fmt.Sprint(rpr.ID)
		pr.RepoID = ids.RepoID(repoID, qc)
		pr.BranchName = rpr.Source.Branch.Name
		pr.Title = rpr.Title
		pr.Description = rpr.Description
		pr.URL = rpr.Links.HTML.Href
		date.ConvertToModel(rpr.CreatedOn, &pr.CreatedDate)
		date.ConvertToModel(rpr.UpdatedOn, &pr.MergedDate)
		date.ConvertToModel(rpr.UpdatedOn, &pr.ClosedDate)
		date.ConvertToModel(rpr.UpdatedOn, &pr.UpdatedDate)
		switch rpr.State {
		case "OPEN":
			pr.Status = sourcecode.PullRequestStatusOpen
		case "DECLINED":
			pr.Status = sourcecode.PullRequestStatusClosed
			pr.ClosedByRefID = rpr.ClosedBy.AccountID
		case "MERGED":
			pr.MergeSha = rpr.MergeCommit.Hash
			pr.MergeCommitID = ids.CodeCommit(qc.CustomerID, qc.RefType, pr.RepoID, rpr.MergeCommit.Hash)
			pr.MergedByRefID = rpr.ClosedBy.AccountID
			pr.Status = sourcecode.PullRequestStatusMerged
		default:
			qc.Logger.Error("PR has an unknown state", "state", rpr.State, "ref_id", pr.RefID)
		}
		pr.CreatedByRefID = rpr.Author.AccountID

		res = append(res, pr)
	}

	return
}
