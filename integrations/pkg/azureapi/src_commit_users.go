package azureapi

import (
	"fmt"
	"net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/commitusers"
)

// FetchCommitUsers gets users from the commits api
// 		returns map[user.Email]*commitusers.CommitUser or Error
func (api *API) FetchCommitUsers(repoids []string, fromdate time.Time) (map[string]*commitusers.CommitUser, error) {
	usermap := make(map[string]*commitusers.CommitUser)
	for _, repoid := range repoids {
		commits, err := api.fetchCommits(repoid, fromdate)
		if err != nil {
			api.logger.Error("[ERROR] error fetching commits. Error", err)
			continue
		}
		for _, commit := range commits {
			if commit.Author.Email != "" {
				if _, ok := usermap[commit.Author.Email]; !ok {
					usermap[commit.Author.Email] = &commitusers.CommitUser{
						CustomerID: api.customerid,
						Name:       commit.Author.Name,
						Email:      commit.Author.Email,
					}
				}
			}
			if commit.Committer.Email != "" {
				if _, ok := usermap[commit.Committer.Email]; !ok {
					usermap[commit.Committer.Email] = &commitusers.CommitUser{
						CustomerID: api.customerid,
						Name:       commit.Committer.Name,
						Email:      commit.Committer.Email,
					}
				}
			}
		}
	}
	return usermap, nil
}

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

func (api *API) fetchCommits(repoid string, fromdate time.Time) ([]commitsResponse, error) {
	u := fmt.Sprintf(`_apis/git/repositories/%s/commits`, url.PathEscape(repoid))
	var res []commitsResponse
	if err := api.getRequest(u, stringmap{
		"searchCriteriapi.fromDate": fromdate.Format(time.RFC3339),
		"$top": "3000",
	}, &res); err != nil {
		return nil, err
	}
	return res, nil
}
