package azureapi

import (
	"fmt"
	purl "net/url"

	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchSourcecodeUsers gets users from the teams and memembers api
// 		returns map[user.UniqueName]*sourcecode.User or Error
//		(user.UniqueName seems to be the email for Azure)
func (api *API) FetchSourcecodeUsers(projids []string) (map[string]*sourcecode.User, error) {
	usermap := make(map[string]*sourcecode.User)
	for _, projid := range projids {
		teams, err := api.fetchTeams(projid)
		if err != nil {
			return nil, err
		}
		rawusermap := make(map[string]*usersResponse)
		for _, team := range teams {
			res, err := api.fetchUsers(projid, team.ID)
			if err != nil {
				return nil, err
			}
			for _, u := range res {
				rawusermap[u.ID] = &u
			}
		}
		for _, u := range rawusermap {
			username := u.UniqueName
			if _, ok := usermap[username]; !ok {
				usermap[username] = &sourcecode.User{
					AvatarURL:  &u.ImageURL,
					CustomerID: api.customerid,
					Member:     true,
					Name:       u.DisplayName,
					RefID:      u.ID,
					RefType:    api.reftype,
					Type:       sourcecode.UserTypeHuman,
					Username:   &username,
				}
			}
		}
	}
	return usermap, nil
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
