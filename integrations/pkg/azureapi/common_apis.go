package azureapi

import (
	"fmt"
	purl "net/url"
)

func (api *API) FetchTeamIDs(projid string) (ids []string, _ error) {
	teams, err := api.fetchTeams(projid)
	if err != nil {
		return ids, err
	}
	for _, team := range teams {
		ids = append(ids, team.ID)
	}
	return
}

func (api *API) fetchAllUsers(projid string, teamids []string) ([]usersResponse, error) {
	usersmap := make(map[string]usersResponse)
	for _, teamid := range teamids {
		users, err := api.fetchUsers(projid, teamid)
		if err != nil {
			return nil, nil
		}
		for _, u := range users {
			usersmap[u.ID] = u
		}
	}
	var users []usersResponse
	for _, u := range usersmap {
		users = append(users, u)
	}
	return users, nil
}

func (api *API) fetchTeams(projid string) ([]teamsResponse, error) {
	url := fmt.Sprintf(`_apis/projects/%s/teams`, purl.PathEscape(projid))
	var res []teamsResponse
	if err := api.getRequest(url, nil, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *API) fetchUsers(projid string, teamid string) ([]usersResponse, error) {
	url := fmt.Sprintf(`_apis/projects/%s/teams/%s/members`, purl.PathEscape(projid), purl.PathEscape(teamid))
	if api.tfs {
		var res []usersResponse
		if err := api.getRequest(url, nil, &res); err != nil {
			return nil, err
		}
		return res, nil
	}
	var res []usersResponseAzure
	if err := api.getRequest(url, nil, &res); err != nil {
		return nil, err
	}
	var users []usersResponse
	for _, r := range res {
		users = append(users, r.Identity)
	}
	return users, nil
}
