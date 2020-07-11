package api

import (
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/pkg/date"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func PullRequestCommentsPage(
	qc QueryContext,
	repo commonrepo.Repo,
	pr sourcecode.PullRequest,
	params url.Values,
	nextPage NextPage) (np NextPage, res []*sourcecode.PullRequestComment, err error) {

	qc.Logger.Debug("pull request comments", "repo", repo.RefID, "repo_name", repo.NameWithOwner, "pr_i", pr.Identifier, "pr_ref_id", pr.RefID, "params", params)

	objectPath := pstrings.JoinURL("repositories", repo.NameWithOwner, "pullrequests", pr.RefID, "comments")

	var rcomments []struct {
		ID    int64 `json:"id"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
		UpdatedOn time.Time `json:"updated_on"`
		CreatedOn time.Time `json:"created_on"`
		Content   struct {
			Raw string `json:"raw"`
		} `json:"content"`
		User struct {
			AccountID string `json:"account_id"`
		} `json:"user"`
		Inline json.RawMessage `json:"inline"`
	}

	np, err = qc.Request(objectPath, params, true, &rcomments, nextPage)
	if err != nil {
		return
	}

	for _, rcomment := range rcomments {
		// ignore reviews comments
		if len(rcomment.Inline) > 0 {
			continue
		}
		item := &sourcecode.PullRequestComment{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = strconv.FormatInt(rcomment.ID, 10)
		item.URL = rcomment.Links.HTML.Href
		date.ConvertToModel(rcomment.UpdatedOn, &item.UpdatedDate)
		item.RepoID = qc.IDs.CodeRepo(repo.RefID)
		item.PullRequestID = qc.IDs.CodePullRequest(item.RepoID, pr.RefID)
		item.Body = rcomment.Content.Raw
		date.ConvertToModel(rcomment.CreatedOn, &item.CreatedDate)
		item.UserRefID = rcomment.User.AccountID
		res = append(res, item)
	}

	return
}
