package api

import (
	"net/url"
	"time"

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
	prIID string,
	params url.Values) (pi PageInfo, res []*sourcecode.PullRequestCommit, err error) {

	qc.Logger.Debug("pull request commits", "repo", repo.NameWithOwner)

	objectPath := pstrings.JoinURL("projects", repo.ID, "merge_requests", prIID, "commits")

	var rcommits []struct {
		ID             string    `json:"id"`
		Message        string    `json:"message"`
		CreatedAt      time.Time `json:"created_at"`
		AuthorEmail    string    `json:"author_email"`
		CommitterEmail string    `json:"committer_email"`
	}

	pi, err = qc.Request(objectPath, params, &rcommits)
	if err != nil {
		return
	}

	for _, rcommit := range rcommits {

		item := &sourcecode.PullRequestCommit{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = rcommit.ID
		item.RepoID = qc.IDs.CodeRepo(repo.ID)
		item.PullRequestID = qc.IDs.CodePullRequest(item.RepoID, prID)
		item.Sha = rcommit.ID
		item.Message = rcommit.Message
		url, err := url.Parse(qc.BaseURL)
		if err != nil {
			return pi, res, err
		}
		item.URL = url.Scheme + "://" + url.Hostname() + "/" + repo.NameWithOwner + "/commit/" + rcommit.ID
		date.ConvertToModel(rcommit.CreatedAt, &item.CreatedDate)

		item.AuthorRefID = ids.CodeCommitEmail(qc.CustomerID, rcommit.AuthorEmail)
		item.CommitterRefID = ids.CodeCommitEmail(qc.CustomerID, rcommit.CommitterEmail)

		res = append(res, item)
	}

	return
}
