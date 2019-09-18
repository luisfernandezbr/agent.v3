package api

import (
	"net/url"

	pstrings "github.com/pinpt/go-common/strings"
)

func PullRequestCommitsPage(
	qc QueryContext,
	repoName string,
	prID string,
	params url.Values) (pi PageInfo, res []string, err error) {

	qc.Logger.Debug("pull request commits", "repo", repoName)

	objectPath := pstrings.JoinURL("repositories", repoName, "pullrequests", prID, "commits")

	var rcommits []struct {
		Hash string `json:"hash"`
	}

	// Setting the page parameter alone as part of params results in "Invalid page" error
	params.Set("fields", "values.hash,page,pagelen,size?page="+params.Get("page"))
	params.Del("page")

	pi, err = qc.Request(objectPath, params, true, &rcommits)
	if err != nil {
		return
	}

	for _, rcommit := range rcommits {
		res = append(res, rcommit.Hash)
	}

	return
}
