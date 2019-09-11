package api

import (
	"fmt"
	purl "net/url"
	"time"
)

type RawCommitResponse struct {
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
	CommitID  string `json:"commitId"`
	Comment   string `json:"comment"`
	Author    struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Date  string `json:"date"`
	} `json:"author"`
	Committer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Date  string `json:"date"`
	} `json:"committer"`
}

// FetchLastCommit calls the commits api to get user information and returns a list of unique sourcecode.User
func (a *TFSAPI) FetchLastCommit(repoid string) (*RawCommitResponse, error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/commits`, purl.PathEscape(repoid))
	var res []RawCommitResponse
	if err := a.doRequest(url, params{"$top": "1"}, time.Time{}, &res); err != nil {
		return nil, err
	}
	if len(res) > 0 {
		return &res[0], nil
	}
	return nil, nil
}
