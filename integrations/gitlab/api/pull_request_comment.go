package api

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/pkg/date"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func PullRequestCommentsPage(
	qc QueryContext,
	repo commonrepo.Repo,
	pr PullRequest,
	params url.Values) (pi PageInfo, res []*sourcecode.PullRequestComment, err error) {

	qc.Logger.Debug("pull request comments", "repo", repo.RefID)

	objectPath := pstrings.JoinURL("projects", url.QueryEscape(repo.RefID), "merge_requests", pr.IID, "notes")

	var rcomments []struct {
		ID     int64 `json:"id"`
		Author struct {
			ID int64 `json:"id"`
		} `json:"author"`
		Body      string    `json:"body"`
		UpdatedAt time.Time `json:"updated_at"`
		CreatedAt time.Time `json:"created_at"`
		System    bool      `json:"system"`
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
		if rcomment.System {
			continue
		}
		item := &sourcecode.PullRequestComment{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = fmt.Sprint(rcomment.ID)
		item.URL = pstrings.JoinURL(u.Scheme, "://", u.Hostname(), repo.NameWithOwner, "merge_requests", pr.IID)
		date.ConvertToModel(rcomment.UpdatedAt, &item.UpdatedDate)
		item.RepoID = qc.IDs.CodeRepo(repo.RefID)
		item.PullRequestID = qc.IDs.CodePullRequest(item.RepoID, pr.ID)
		item.Body = rcomment.Body
		date.ConvertToModel(rcomment.CreatedAt, &item.CreatedDate)

		item.UserRefID = strconv.FormatInt(rcomment.Author.ID, 10)
		res = append(res, item)
	}

	return
}
