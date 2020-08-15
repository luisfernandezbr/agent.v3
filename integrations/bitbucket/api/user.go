package api

import (
	"net/url"

	"github.com/pinpt/go-common/strings"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func UsersSourcecodePage(
	qc QueryContext,
	group string,
	params url.Values,
	nextPage NextPage) (np NextPage, users []*sourcecode.User, err error) {

	qc.Logger.Debug("users request", "group", group)

	objectPath := pstrings.JoinURL("workspaces", group, "members")

	var us []struct {
		User struct {
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
		} `json:"user"`
	}

	np, err = qc.Request(objectPath, params, true, &us, nextPage)
	if err != nil {
		return
	}

	for _, u := range us {
		user := &sourcecode.User{
			RefID:      u.User.AccountID,
			RefType:    qc.RefType,
			CustomerID: qc.CustomerID,
			Name:       u.User.DisplayName,
			AvatarURL:  pstrings.Pointer(u.User.Links.Avatar.Href),
			Member:     true,
			Type:       sourcecode.UserTypeHuman,
			URL:        strings.Pointer(u.User.Links.HTML.Href),
			// Email: Not available
			// Username: Not available
		}

		users = append(users, user)
	}

	return
}

// AreUserCredentialsValid if this returns ok the credentials are fine
func AreUserCredentialsValid(qc QueryContext) (err error) {

	qc.Logger.Debug("users credentials validation")

	objectPath := pstrings.JoinURL("user")

	var us interface{}

	_, err = qc.Request(objectPath, nil, true, &us, "")

	return
}
