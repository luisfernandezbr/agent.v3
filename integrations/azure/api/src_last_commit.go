package api

import (
	"fmt"
	"net/url"
)

// FetchLastCommit gets the last commit in a repo
func (api *API) FetchLastCommit(repoid string) (*CommitResponse, error) {
	u := fmt.Sprintf(`_apis/git/repositories/%s/commits`, url.PathEscape(repoid))
	var res []CommitResponse
	if err := api.getRequest(u, stringmap{"$top": "1"}, &res); err != nil {
		return nil, err
	}
	if len(res) > 0 {
		return &res[0], nil
	}
	return nil, nil
}
