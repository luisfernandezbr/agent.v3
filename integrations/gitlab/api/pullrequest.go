package api

import (
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent/integrations/pkg/commonpr"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/ids"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type PullRequest struct {
	*sourcecode.PullRequest
	IID           string
	LastCommitSHA string
}

func PullRequestPage(
	qc QueryContext,
	repo commonrepo.Repo,
	params url.Values,
	stopOnUpdatedAt time.Time) (pi PageInfo, res []PullRequest, err error) {

	qc.Logger.Debug("repo pull requests", "repo_ref_id", repo.RefID, "repo", repo.NameWithOwner, "stop_on_updated_at", stopOnUpdatedAt.String(), "params", params)

	objectPath := pstrings.JoinURL("projects", url.QueryEscape(repo.RefID), "merge_requests")
	params.Set("scope", "all")
	params.Set("state", "all")

	var rprs []struct {
		ID           int64     `json:"id"`
		IID          int64     `json:"iid"`
		UpdatedAt    time.Time `json:"updated_at"`
		CreatedAt    time.Time `json:"created_at"`
		ClosedAt     time.Time `json:"closed_at"`
		MergedAt     time.Time `json:"merged_at"`
		SourceBranch string    `json:"source_branch"`
		Title        string    `json:"title"`
		Description  string    `json:"description"`
		WebURL       string    `json:"web_url"`
		State        string    `json:"state"`
		Draft        bool      `json:"work_in_progress"`
		Author       struct {
			ID int64 `json:"id"`
		} `json:"author"`
		ClosedBy struct {
			ID int64 `json:"id"`
		} `json:"closed_by"`
		MergedBy struct {
			ID int64 `json:"id"`
		} `json:"merged_by"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		References     struct {
			Full string `json:"full"`
		} `json:"references"`
		Labels []string `json:"labels"`
	}

	pi, err = qc.Request(objectPath, params, &rprs)
	if err != nil {
		return
	}

	for _, rpr := range rprs {
		if rpr.UpdatedAt.Before(stopOnUpdatedAt) {
			return pi, res, nil
		}
		pr := &sourcecode.PullRequest{}
		pr.CustomerID = qc.CustomerID
		pr.RefType = qc.RefType
		pr.RefID = strconv.FormatInt(rpr.ID, 10)
		pr.RepoID = qc.IDs.CodeRepo(repo.RefID)
		pr.BranchName = rpr.SourceBranch
		pr.Title = rpr.Title
		pr.Description = commonpr.ConvertMarkdownToHTML(rpr.Description)
		pr.URL = rpr.WebURL
		pr.Identifier = rpr.References.Full
		date.ConvertToModel(rpr.CreatedAt, &pr.CreatedDate)
		date.ConvertToModel(rpr.MergedAt, &pr.MergedDate)
		date.ConvertToModel(rpr.ClosedAt, &pr.ClosedDate)
		date.ConvertToModel(rpr.UpdatedAt, &pr.UpdatedDate)
		switch rpr.State {
		case "opened":
			pr.Status = sourcecode.PullRequestStatusOpen
		case "closed":
			pr.Status = sourcecode.PullRequestStatusClosed
			pr.ClosedByRefID = strconv.FormatInt(rpr.ClosedBy.ID, 10)
		case "locked":
			pr.Status = sourcecode.PullRequestStatusLocked
		case "merged":
			pr.MergeSha = rpr.MergeCommitSHA
			pr.MergeCommitID = ids.CodeCommit(qc.CustomerID, qc.RefType, pr.RepoID, rpr.MergeCommitSHA)
			pr.MergedByRefID = strconv.FormatInt(rpr.MergedBy.ID, 10)
			pr.Status = sourcecode.PullRequestStatusMerged
		default:
			qc.Logger.Error("PR has an unknown state", "state", rpr.State, "ref_id", pr.RefID)
		}
		pr.CreatedByRefID = strconv.FormatInt(rpr.Author.ID, 10)
		pr.Draft = rpr.Draft
		pr.Labels = rpr.Labels

		spr := PullRequest{}
		spr.IID = strconv.FormatInt(rpr.IID, 10)
		spr.PullRequest = pr
		res = append(res, spr)
	}

	return
}
