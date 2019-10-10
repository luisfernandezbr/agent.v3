package api

import (
	"github.com/pinpt/integration-sdk/work"
)

// FetchWorkUsers gets all users from all the teams from a single project
func (api *API) FetchWorkUsers(projid string, teamids []string) (users []*work.User, err error) {
	rawusers, err := api.fetchAllUsers(projid, teamids)
	if err != nil {
		return nil, err
	}
	for _, u := range rawusers {
		users = append(users, &work.User{
			AvatarURL:  &u.ImageURL,
			CustomerID: api.customerid,
			Name:       u.DisplayName,
			RefID:      u.ID,
			RefType:    api.reftype,
			Username:   u.UniqueName,
		})
	}
	return
}
