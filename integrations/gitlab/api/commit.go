package api

import (
	"net/url"

	pstrings "github.com/pinpt/go-common/strings"
)

type CommitAuthor struct {
	CommitHash  string
	AuthorName  string
	AuthorEmail string
	// AuthorRefID    string
	CommitterName  string
	CommitterEmail string
	// CommitterRefID string
}

func CommitsPage(
	qc QueryContext,
	repoRefID string, branchName string,
	params url.Values) (pi PageInfo, res []CommitAuthor, err error) {
	qc.Logger.Debug("repos commits", "repoID", repoRefID, "branch", branchName)

	objectPath := pstrings.JoinURL("projects", repoRefID, "repository", "commits")

	params.Set("ref_name", branchName)

	var rc []struct {
		SHA            string `json:"id"`
		AuthorName     string `json:"author_name"`
		AuthorEmail    string `json:"author_email"`
		CommitterName  string `json:"committer_name"`
		CommitterEmail string `json:"committer_email"`
	}

	pi, err = qc.Request(objectPath, params, &rc)
	if err != nil {
		return
	}

	for _, c := range rc {
		item := CommitAuthor{}
		item.CommitHash = c.SHA
		item.AuthorName = c.AuthorName
		item.AuthorEmail = c.AuthorEmail
		item.CommitterName = c.CommitterName
		item.CommitterEmail = c.CommitterEmail
		res = append(res, item)
	}

	return
}
