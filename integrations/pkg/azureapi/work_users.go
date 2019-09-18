package azureapi

import (
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/work"
)

func (api *API) FetchWorkUsers(repoids []string, userchan chan<- datamodel.Model) error {
	srcusers, err := api.FetchSourcecodeUsers(repoids)
	if err != nil {
		return err
	}
	for _, u := range srcusers {
		userchan <- &work.User{
			AvatarURL:  u.AvatarURL,
			CustomerID: api.customerid,
			Name:       u.Name,
			RefID:      u.RefID,
			RefType:    api.reftype,
			Username:   *u.Username,
		}
	}
	return nil
}
