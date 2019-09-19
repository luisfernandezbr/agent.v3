package api

import (
	"fmt"
	"net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/commonrepo"
	"github.com/pinpt/agent.next/pkg/date"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func PullRequestCommentsPage(
	qc QueryContext,
	repo commonrepo.Repo,
	pr PullRequest,
	params url.Values) (pi PageInfo, res []*sourcecode.PullRequestComment, err error) {

	qc.Logger.Debug("pull request commits", "repo", repo.ID)

	objectPath := pstrings.JoinURL("projects", url.QueryEscape(repo.ID), "merge_requests", pr.IID, "notes")

	var rcomments []struct {
		ID     int64 `json:"id"`
		Author struct {
			Username string `json:"username"`
		}
		Body      string    `json:"body"`
		UpdatedAt time.Time `json:"updated_at"`
		CreatedAt time.Time `json:"created_at"`
	}

	pi, err = qc.Request(objectPath, params, &rcomments)
	if err != nil {
		return
	}

	u, err := url.Parse(qc.BaseURL)
	if err != nil {
		return pi, res, err
	}

	for _, rcomment := range rcomments {
		item := &sourcecode.PullRequestComment{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = fmt.Sprint(rcomment.ID)
		item.URL = pstrings.JoinURL(u.Scheme, "://", u.Hostname(), repo.NameWithOwner, "merge_requests", pr.IID)
		date.ConvertToModel(rcomment.UpdatedAt, &item.UpdatedDate)
		item.RepoID = qc.BasicInfo.RepoID(repo.ID)
		item.PullRequestID = qc.BasicInfo.PullRequestID(repo.ID, pr.ID)
		item.Body = rcomment.Body
		date.ConvertToModel(rcomment.CreatedAt, &item.CreatedDate)

		item.UserRefID = rcomment.Author.Username
		res = append(res, item)
	}

	return
}
