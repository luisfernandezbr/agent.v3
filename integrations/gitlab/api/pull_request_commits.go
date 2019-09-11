package api

import (
	"net/url"

	pstrings "github.com/pinpt/go-common/strings"
)

func PullRequestCommitsPage(
	qc QueryContext,
	repoRefID string,
	prIID string,
	params url.Values) (pi PageInfo, res []string, err error) {

	qc.Logger.Debug("pull request commits", "repo", repoRefID)

	objectPath := pstrings.JoinURL("projects", url.QueryEscape(repoRefID), "merge_requests", prIID, "commits")

	var rcommits []struct {
		ID string `json:"id"`
	}

	pi, err = qc.Request(objectPath, params, &rcommits)
	if err != nil {
		return
	}

	for _, rcommit := range rcommits {
		res = append(res, rcommit.ID)
	}

	return
}
