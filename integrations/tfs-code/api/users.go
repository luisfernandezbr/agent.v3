package api

import (
	"fmt"
	purl "net/url"
	"time"

	"github.com/pinpt/integration-sdk/sourcecode"
)

type teamsResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"identityUrl"`
	Description string `json:"description"`
}

func (a *TFSAPI) fetchTeams(projid string) ([]teamsResponse, error) {
	url := fmt.Sprintf(`_apis/projects/%s/teams`, purl.PathEscape(projid))
	var res []teamsResponse
	if err := a.doRequest(url, nil, time.Time{}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

type usersResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	UniqueName  string `json:"uniqueName"`
	URL         string `json:"url"`
	ImageURL    string `json:"imageUrl"`
}

func (a *TFSAPI) fetchUsers(projid string, teamid string) ([]usersResponse, error) {
	url := fmt.Sprintf(`_apis/projects/%s/teams/%s/members`, purl.PathEscape(projid), purl.PathEscape(teamid))
	var res []usersResponse
	if err := a.doRequest(url, nil, time.Time{}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (a *TFSAPI) FetchUsers(projid string, usermap map[string]*sourcecode.User) error {
	teams, err := a.fetchTeams(projid)
	if err != nil {
		return fmt.Errorf("error fetching teams. error %v", err)
	}
	for _, t := range teams {
		usrs, err := a.fetchUsers(projid, t.ID)
		if err != nil {
			return fmt.Errorf("error fetching users. error %v", err)
		}
		for _, u := range usrs {
			usermap[u.ID] = &sourcecode.User{
				AvatarURL:  &u.ImageURL,
				CustomerID: a.customerid,
				Member:     true,
				Name:       u.DisplayName,
				RefID:      u.ID,
				RefType:    a.reftype,
				Type:       sourcecode.UserTypeHuman,
				Username:   &u.UniqueName,
			}
		}
	}
	return nil
}
