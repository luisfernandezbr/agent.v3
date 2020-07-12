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
	logger hclog.Logger,
	repo commonrepo.Repo,
	pr sourcecode.PullRequest,
	params url.Values,
	stopOnUpdatedAt time.Time,
	nextPage NextPage) (np NextPage, res []*sourcecode.PullRequestCommit, err error) {

	logger.Debug("pr commits", "inc_date", stopOnUpdatedAt, "params", params, "next_page", nextPage)

	objectPath := pstrings.JoinURL("repositories", repo.NameWithOwner, "pullrequests", pr.RefID, "commits")

	var rcommits []struct {
		Hash    string    `json:"hash"`
		Message string    `json:"message"`
		Date    time.Time `json:"date"`
		Author  struct {
			Raw string `json:"raw"`
		} `json:"author"`
	}

	np, err = qc.Request(objectPath, params, true, &rcommits, nextPage)
	if err != nil {
		return
	}

	for _, rcommit := range rcommits {
		if rcommit.Date.Before(stopOnUpdatedAt) {
			np = ""
			return
		}
		item := &sourcecode.PullRequestCommit{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = rcommit.Hash
		item.RepoID = qc.IDs.CodeRepo(repo.RefID)
		item.PullRequestID = qc.IDs.CodePullRequest(item.RepoID, pr.RefID)
		item.Sha = rcommit.Hash
		item.Message = rcommit.Message
		url, err := url.Parse(qc.BaseURL)
		if err != nil {
			return np, res, err
		}
		item.URL = url.Scheme + "://" + strings.TrimPrefix(url.Hostname(), "api.") + "/" + repo.NameWithOwner + "/commits/" + rcommit.Hash
		date.ConvertToModel(rcommit.Date, &item.CreatedDate)

		_, authorEmail := GetNameAndEmail(rcommit.Author.Raw)

		item.AuthorRefID = ids.CodeCommitEmail(qc.CustomerID, authorEmail)
		item.CommitterRefID = ids.CodeCommitEmail(qc.CustomerID, authorEmail)

		res = append(res, item)
	}

	return
}
