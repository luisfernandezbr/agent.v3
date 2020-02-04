package api

import (
	"strings"

	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchSourcecodeUsers gets users from the teams and memembers api, passing in a map to avoid dups
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
			AvatarURL:  pstrings.Pointer(user.ImageURL),
			CustomerID: api.customerid,
			Member:     true,
			Name:       user.DisplayName,
			RefID:      user.ID,
			RefType:    api.reftype,
			Type:       usertype,
			Username:   pstrings.Pointer(user.UniqueName),
		}
	}
	return nil
}
