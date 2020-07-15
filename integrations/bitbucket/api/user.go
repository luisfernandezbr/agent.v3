package api

import (
	"net/url"

	"github.com/pinpt/go-common/v10/strings"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func UsersSourcecodePage(
	qc QueryContext,
	group string,
	params url.Values,
	nextPage NextPage) (np NextPage, users []*sourcecode.User, err error) {

	qc.Logger.Debug("users request", "group", group)

	objectPath := pstrings.JoinURL("teams", group, "members")

	var us []struct {
		DisplayName string `json:"display_name"`
		Links       struct {
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
		AccountID string `json:"account_id"`
	}

	np, err = qc.Request(objectPath, params, true, &us, nextPage)
	if err != nil {
		return
	}

	for _, u := range us {
		user := &sourcecode.User{
			RefID:      u.AccountID,
			RefType:    qc.RefType,
			CustomerID: qc.CustomerID,
			Name:       u.DisplayName,
			AvatarURL:  pstrings.Pointer(u.Links.Avatar.Href),
			Member:     true,
			Type:       sourcecode.UserTypeHuman,
			URL:        strings.Pointer(u.Links.HTML.Href),
			// Email: Not possible
			// Username: Not possible
		}

		users = append(users, user)
	}

	return
}
