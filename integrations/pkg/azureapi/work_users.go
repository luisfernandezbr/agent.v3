package azureapi

import (
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/work"
)

// FetchWorkUsers gets all users from all the teams from a single project
func (api *API) FetchWorkUsers(projid string, teamids []string, userchan chan<- datamodel.Model) error {
	users, err := api.fetchAllUsers(projid, teamids)
	if err != nil {
		return err
	}
	for _, u := range users {
		userchan <- &work.User{
			AvatarURL:  &u.ImageURL,
			CustomerID: api.customerid,
			Name:       u.DisplayName,
			RefID:      u.ID,
			RefType:    api.reftype,
			Username:   u.UniqueName,
		}
	}
	return nil
}
