package api

import (
	"net/url"

	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/go-datamodel/sourcecode"
	"github.com/pinpt/integration-sdk/agent"
)

func UsersOnboardPage(qc QueryContext, teamName string, params url.Values) (page PageInfo, users []*agent.UserResponseUsers, err error) {
	qc.Logger.Debug("onboard repos request")

	objectPath := pstrings.JoinURL("teams", teamName, "members")
	params.Set("pagelen", "100")

	var rusers []struct {
		AccountID string `json:"account_id"`
		Links     struct {
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"links"`
		DisplayName string `json:"display_name"`
	}

	page, err = qc.Request(objectPath, params, true, &rusers)
	if err != nil {
		return
	}

	for _, u := range rusers {
		repo := &agent.UserResponseUsers{
			RefID:      u.AccountID,
			RefType:    qc.RefType,
			CustomerID: qc.CustomerID,
			AvatarURL:  pstrings.Pointer(u.Links.Avatar.Href),
			Active:     true,
			Name:       u.DisplayName,
		}

		users = append(users, repo)
	}

	return
}

func UsersSourcecodePage(qc QueryContext, group string, params url.Values) (page PageInfo, users []*sourcecode.User, err error) {
	qc.Logger.Debug("users request", "group", group)

	objectPath := pstrings.JoinURL("teams", group, "members")

	params.Set("pagelen", "100")

	var us []struct {
		UUID        string `json:"uuid"`
		DisplayName string `json:"display_name"`
		Links       struct {
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"links"`
		AccountID string `json:"account_id"`
	}

	page, err = qc.Request(objectPath, params, true, &us)
	if err != nil {
		return
	}

	for _, u := range us {
		user := &sourcecode.User{
			RefID:           u.UUID,
			RefType:         qc.RefType,
			CustomerID:      qc.CustomerID,
			Name:            u.DisplayName,
			AvatarURL:       pstrings.Pointer(u.Links.Avatar.Href),
			Member:          true,
			Type:            sourcecode.UserTypeHuman,
			AssociatedRefID: pstrings.Pointer(u.AccountID),
			// Email: Not possible
			// Username: Not possible
		}

		users = append(users, user)
	}

	return
}
