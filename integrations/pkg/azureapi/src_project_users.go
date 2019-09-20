package azureapi

import (
	"strings"

	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchSourcecodeUsers gets users from the teams and memembers api
// 		returns map[user.UniqueName]*sourcecode.User or Error
//		(user.UniqueName seems to be the email for Azure)
func (api *API) FetchSourcecodeUsers(projid string, teamids []string, usermap map[string]*sourcecode.User) error {
	allusers, err := api.fetchAllUsers(projid, teamids)
	if err != nil {
		return err
	}
	for _, user := range allusers {
		var usertype sourcecode.UserType
		if strings.Contains(user.DisplayName, `]\\`) {
			usertype = sourcecode.UserTypeBot
		} else {
			usertype = sourcecode.UserTypeHuman
		}
		usermap[user.UniqueName] = &sourcecode.User{
			AvatarURL:  &user.ImageURL,
			CustomerID: api.customerid,
			Member:     true,
			Name:       user.DisplayName,
			RefID:      user.ID,
			RefType:    api.reftype,
			Type:       usertype,
			Username:   &user.UniqueName,
		}
	}
	return nil
}
