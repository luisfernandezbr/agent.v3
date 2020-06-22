package api

import (
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/ids"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
	"github.com/russross/blackfriday"
)

type PullRequest struct {
	*sourcecode.PullRequest
	IID           string
	LastCommitSHA string
}

func PullRequestPage(
	qc QueryContext,
	repoRefID string,
	params url.Values,
	stopOnUpdatedAt time.Time) (pi PageInfo, res []PullRequest, err error) {

	qc.Logger.Debug("repo pull requests", "repo", repoRefID)

	objectPath := pstrings.JoinURL("projects", url.QueryEscape(repoRefID), "merge_requests")
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
			Username string `json:"username"`
		} `json:"author"`
		ClosedBy struct {
			Username string `json:"username"`
		} `json:"closed_by"`
		MergedBy struct {
			Username string `json:"username"`
		} `json:"merged_by"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		Identifier     string `json:"reference"` // this looks how we display in Gitlab such as !1
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
		pr.RepoID = qc.IDs.CodeRepo(repoRefID)
		pr.BranchName = rpr.SourceBranch
		pr.Title = rpr.Title
		pr.Description = convertMarkdownToHTML(rpr.Description)
		pr.URL = rpr.WebURL
		pr.Identifier = rpr.Identifier
		date.ConvertToModel(rpr.CreatedAt, &pr.CreatedDate)
		date.ConvertToModel(rpr.MergedAt, &pr.MergedDate)
		date.ConvertToModel(rpr.ClosedAt, &pr.ClosedDate)
		date.ConvertToModel(rpr.UpdatedAt, &pr.UpdatedDate)
		switch rpr.State {
		case "opened":
			pr.Status = sourcecode.PullRequestStatusOpen
		case "closed":
			pr.Status = sourcecode.PullRequestStatusClosed
			pr.ClosedByRefID = rpr.ClosedBy.Username
		case "locked":
			pr.Status = sourcecode.PullRequestStatusLocked
		case "merged":
			pr.MergeSha = rpr.MergeCommitSHA
			pr.MergeCommitID = ids.CodeCommit(qc.CustomerID, qc.RefType, pr.RepoID, rpr.MergeCommitSHA)
			pr.MergedByRefID = rpr.MergedBy.Username
			pr.Status = sourcecode.PullRequestStatusMerged
		default:
			qc.Logger.Error("PR has an unknown state", "state", rpr.State, "ref_id", pr.RefID)
		}
		pr.CreatedByRefID = rpr.Author.Username
		pr.Draft = rpr.Draft

		spr := PullRequest{}
		spr.IID = strconv.FormatInt(rpr.IID, 10)
		spr.PullRequest = pr
		res = append(res, spr)
	}

	return
}

const extensions = blackfriday.NoIntraEmphasis |
	blackfriday.Tables |
	blackfriday.FencedCode |
	blackfriday.Autolink |
	blackfriday.Strikethrough |
	blackfriday.SpaceHeadings |
	blackfriday.NoEmptyLineBeforeBlock

func convertMarkdownToHTML(text string) string {
	output := blackfriday.Run([]byte(text), blackfriday.WithExtensions(extensions))
	return string(output)
}
