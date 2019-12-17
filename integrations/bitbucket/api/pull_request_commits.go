package api

import (
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/integration-sdk/sourcecode"

	pstrings "github.com/pinpt/go-common/strings"
)

func PullRequestCommitsPage(
	qc QueryContext,
	repo commonrepo.Repo,
	prID string,
	params url.Values) (pi PageInfo, res []*sourcecode.PullRequestCommit, err error) {

	qc.Logger.Debug("pull request commits", "repo", repo.NameWithOwner, "pr", prID)

	objectPath := pstrings.JoinURL("repositories", repo.NameWithOwner, "pullrequests", prID, "commits")

	var rcommits []struct {
		Hash    string    `json:"hash"`
		Message string    `json:"message"`
		Date    time.Time `json:"date"`
		Author  struct {
			Raw string `json:"raw"`
		} `json:"author"`
	}

	params.Set("pagelen", "100")
	// Setting the page parameter alone as part of params results in "Invalid page" error
	params.Set("fields", "values.hash,values.message,values.date,values.author.raw,page,pagelen,size?page="+params.Get("page"))
	params.Del("page")

	pi, err = qc.Request(objectPath, params, true, &rcommits)
	if err != nil {
		return
	}

	for _, rcommit := range rcommits {
		item := &sourcecode.PullRequestCommit{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = rcommit.Hash
		item.RepoID = qc.IDs.CodeRepo(repo.ID)
		item.PullRequestID = qc.IDs.CodePullRequest(repo.ID, prID)
		item.Sha = rcommit.Hash
		item.Message = rcommit.Message
		url, err := url.Parse(qc.BaseURL)
		if err != nil {
			return pi, res, err
		}
		item.URL = url.Scheme + "://" + strings.TrimPrefix(url.Hostname(), "api.") + "/" + repo.NameWithOwner + "/commits/" + rcommit.Hash
		date.ConvertToModel(rcommit.Date, &item.CreatedDate)

		adds, dels, err := commitStats(qc, repo.NameWithOwner, rcommit.Hash)
		if err != nil {
			return pi, res, err
		}

		item.Additions = adds
		item.Deletions = dels

		_, authorEmail := GetNameAndEmail(rcommit.Author.Raw)

		item.AuthorRefID = ids.CodeCommitEmail(qc.CustomerID, authorEmail)
		item.CommitterRefID = ids.CodeCommitEmail(qc.CustomerID, authorEmail)

		res = append(res, item)
	}

	return
}

type stats struct {
	Additions int64
	Deletions int64
}

func CommitStatsPage(
	qc QueryContext,
	repoName string,
	commitID string,
	params url.Values) (pi PageInfo, res []stats, err error) {

	qc.Logger.Debug("commit stat", "repo", repoName, "commit", commitID)

	objectPath := pstrings.JoinURL("repositories", repoName, "diffstat", commitID)

	var rfiles []struct {
		LinesRemoved int64 `json:"lines_removed"`
		LinesAdded   int64 `json:"lines_added"`
	}

	params.Set("pagelen", "100")

	pi, err = qc.Request(objectPath, params, true, &rfiles)
	if err != nil {
		return
	}

	for _, rfile := range rfiles {
		item := stats{
			Additions: rfile.LinesAdded,
			Deletions: rfile.LinesRemoved,
		}

		res = append(res, item)
	}

	return
}

func commitStats(qc QueryContext, repoName string, commitID string) (adds, dels int64, err error) {
	Paginate(qc.Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
		pi, res, err := CommitStatsPage(qc, repoName, commitID, paginationParams)
		if err != nil {
			return pi, err
		}

		for _, obj := range res {
			adds += obj.Additions
			dels += obj.Deletions
		}
		return pi, nil
	})
	return
}
