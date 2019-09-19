package api

import (
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent.next/pkg/commonrepo"
	"github.com/pinpt/agent.next/pkg/date"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func PullRequestCommentsPage(
	qc QueryContext,
	repo commonrepo.Repo,
	pr sourcecode.PullRequest,
	params url.Values) (pi PageInfo, res []*sourcecode.PullRequestComment, err error) {

	qc.Logger.Debug("pull request commits", "repo", repo.ID)

	params.Set("pagelen", "100")

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
	}

	pi, err = qc.Request(objectPath, params, true, &rcomments)
	if err != nil {
		return
	}

	for _, rcomment := range rcomments {
		item := &sourcecode.PullRequestComment{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = strconv.FormatInt(rcomment.ID, 10)
		item.URL = rcomment.Links.HTML.Href
		date.ConvertToModel(rcomment.UpdatedOn, &item.UpdatedDate)
		item.UpdatedAt = rcomment.UpdatedOn.Unix()
		item.RepoID = qc.BasicInfo.RepoID(repo.ID)
		item.PullRequestID = qc.BasicInfo.PullRequestID(repo.ID, pr.ID)
		item.Body = rcomment.Content.Raw
		date.ConvertToModel(rcomment.CreatedOn, &item.CreatedDate)
		item.UserRefID = rcomment.User.AccountID
		res = append(res, item)
	}

	return
}
