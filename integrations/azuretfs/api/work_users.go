package api

import (
	"regexp"

	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/work"
)

var doubleSlashRegex = regexp.MustCompile(`^(.*?)\\\\`)

// FetchWorkUsers gets all users from all the teams from a single project
func (api *API) FetchWorkUsers(projid string, teamids []string) (users []*work.User, err error) {
	rawusers, err := api.fetchAllUsers(projid, teamids)
	if err != nil {
		return nil, err
	}
	for _, u := range rawusers {
		users = append(users, &work.User{
			AvatarURL:  pstrings.Pointer(u.ImageURL),
			CustomerID: api.customerid,
			Name:       doubleSlashRegex.ReplaceAllString(u.DisplayName, ""),
			RefID:      u.ID,
			RefType:    api.reftype,
			Username:   doubleSlashRegex.ReplaceAllString(u.UniqueName, ""),
		})
	}
	return
}
