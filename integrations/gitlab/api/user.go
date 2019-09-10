package api

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/go-hclog"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

type User struct {
	Email    string
	Username string
}

func GroupUsers(qc QueryContext, group string) (users []*agent.UserResponseUsers, err error) {
	qc.Logger.Debug("fetching users", "group", group)

	objectPath := pstrings.JoinURL("/groups/", url.PathEscape(group), "/members/all")

	var rawUsers []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}

	_, err = qc.Request(objectPath, nil, &rawUsers)
	if err != nil {
		return users, err
	}

	for _, user := range rawUsers {
		nUser := &agent.UserResponseUsers{
			RefID:      fmt.Sprint(user.ID),
			RefType:    qc.RefType,
			CustomerID: qc.CustomerID,
			Username:   user.Username,
			AvatarURL:  pstrings.Pointer(user.AvatarURL),
			Active:     true,
			Name:       user.Name,
			Groups: []agent.UserResponseUsersGroups{
				agent.UserResponseUsersGroups{
					Name: group,
				},
			},
		}

		nUser.Emails, err = emailsUser(qc, nUser.Username)
		if err != nil {
			return
		}

		users = append(users, nUser)

	}

	return users, nil
}

func emailsUser(qc QueryContext, username string) (emails []string, err error) {

	qc.Logger.Debug("fetching user email", "user", username)

	objectPath := pstrings.JoinURL("users?username=" + username)

	var rawEmails []struct {
		Email string `json:"email"`
	}

	_, err = qc.Request(objectPath, nil, &rawEmails)
	if err != nil {
		return
	}

	for _, email := range rawEmails {
		emails = append(emails, email.Email)
	}

	return
}

func UsersOnboardPage(qc QueryContext, params url.Values) (page PageInfo, users []*agent.UserResponseUsers, err error) {
	qc.Logger.Debug("users request")

	objectPath := pstrings.JoinURL("/users")

	var rawUsers []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}

	params.Set("membership", "true")
	params.Set("per_page", "100")

	page, err = qc.Request(objectPath, params, &rawUsers)
	if err != nil {
		return
	}

	for _, user := range rawUsers {
		nUser := &agent.UserResponseUsers{
			RefID:      fmt.Sprint(user.ID),
			RefType:    qc.RefType,
			CustomerID: qc.CustomerID,
			Username:   user.Username,
			AvatarURL:  pstrings.Pointer(user.AvatarURL),
			Active:     true,
			Name:       user.Name,
			Emails:     []string{user.Email},
		}

		users = append(users, nUser)

	}

	return
}

func UsersEmailsMap(qc QueryContext, params url.Values) (page PageInfo, users []User, err error) {
	qc.Logger.Debug("users request")

	objectPath := pstrings.JoinURL("/users")

	var rawUsers []struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}

	params.Set("membership", "true")
	params.Set("per_page", "100")

	page, err = qc.Request(objectPath, params, &rawUsers)
	if err != nil {
		return
	}

	for _, user := range rawUsers {
		nUser := User{
			Email:    user.Email,
			Username: user.Username,
		}

		users = append(users, nUser)

	}

	return
}

// UserEmailMap ...
func UserEmailMap(qc QueryContext) (m map[string]string, e error) {
	m = make(map[string]string)
	e = PaginateStartAt(qc.Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
		page, users, err := UsersEmailsMap(qc, paginationParams)
		if err != nil {
			return page, err
		}
		for _, user := range users {
			m[user.Email] = user.Username
		}
		return page, nil
	})

	return

}
