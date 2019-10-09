package api

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"

	"github.com/pinpt/agent.next/pkg/commitusers"
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/objsender"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

type User struct {
	ID        int64
	Email     string
	Username  string
	Name      string
	AvatarURL string
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

func UsersPage(qc QueryContext, params url.Values) (page PageInfo, users []User, err error) {
	qc.Logger.Debug("users request")

	objectPath := pstrings.JoinURL("/users")

	var rawUsers []struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		ID        int64  `json:"id"`
		AvatarURL string `json:"avatar_url"`
	}

	params.Set("membership", "true")
	params.Set("per_page", "100")

	page, err = qc.Request(objectPath, params, &rawUsers)
	if err != nil {
		return
	}

	for _, user := range rawUsers {
		nUser := User{
			Email:     user.Email,
			Username:  user.Username,
			Name:      user.Name,
			ID:        user.ID,
			AvatarURL: user.AvatarURL,
		}

		users = append(users, nUser)

	}

	return
}

func UserEmails(qc QueryContext, userID int64) (emails []string, err error) {
	qc.Logger.Debug("users request")

	objectPath := pstrings.JoinURL("users", fmt.Sprint(userID), "emails")

	var rawEmails []struct {
		Email string `json:"email"`
	}

	_, err = qc.Request(objectPath, nil, &rawEmails)
	if err != nil {
		return
	}

	for _, user := range rawEmails {
		emails = append(emails, user.Email)
	}

	return
}

func UsersEmails(qc QueryContext, commitUsersSender *objsender.IncrementalDateBased, usersSender *objsender.NotIncremental) error {
	return PaginateStartAt(qc.Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
		page, users, err := UsersPage(qc, paginationParams)
		if err != nil {
			return page, err
		}
		for _, user := range users {
			cUser := commitusers.CommitUser{
				CustomerID: qc.CustomerID,
				Email:      user.Email,
				Name:       user.Name,
				SourceID:   user.Username,
			}
			err = cUser.Validate()
			if err != nil {
				return page, err
			}

			if err := commitUsersSender.SendMap(cUser.ToMap()); err != nil {
				return page, err
			}

			emails, err := UserEmails(qc, user.ID)
			if err != nil {
				return page, err
			}
			for _, email := range emails {
				cUser := commitusers.CommitUser{
					CustomerID: qc.CustomerID,
					Email:      email,
					Name:       user.Name,
					SourceID:   user.Username,
				}
				err := cUser.Validate()
				if err != nil {
					return page, err
				}

				if err := commitUsersSender.SendMap(cUser.ToMap()); err != nil {
					return page, err
				}
			}

			sourceUser := sourcecode.User{}
			sourceUser.RefType = qc.RefType
			sourceUser.Email = pstrings.Pointer(user.Email)
			sourceUser.CustomerID = qc.CustomerID
			sourceUser.RefID = strconv.FormatInt(user.ID, 10)
			sourceUser.Name = user.Name
			sourceUser.AvatarURL = pstrings.Pointer(user.AvatarURL)
			sourceUser.Username = pstrings.Pointer(user.Username)
			sourceUser.Member = true
			sourceUser.Type = sourcecode.UserTypeHuman
			sourceUser.AssociatedRefID = pstrings.Pointer(user.Username)

			if err := usersSender.Send(&sourceUser); err != nil {
				return page, err
			}

		}

		return page, nil
	})
}

func RepoUsersPageREST(qc QueryContext, repo commonrepo.Repo, params url.Values) (page PageInfo, repos []*sourcecode.User, err error) {
	qc.Logger.Debug("users request", "repo", repo)

	objectPath := pstrings.JoinURL("projects", repo.ID, "users")

	var ru []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}

	page, err = qc.Request(objectPath, params, &ru)
	if err != nil {
		return
	}

	for _, user := range ru {
		sourceUser := sourcecode.User{}
		sourceUser.RefType = qc.RefType
		// sourceUser.Email = // No email info here
		sourceUser.CustomerID = qc.CustomerID
		sourceUser.RefID = strconv.FormatInt(user.ID, 10)
		sourceUser.Name = user.Name
		sourceUser.AvatarURL = pstrings.Pointer(user.AvatarURL)
		sourceUser.Username = pstrings.Pointer(user.Username)
		sourceUser.Member = true
		sourceUser.Type = sourcecode.UserTypeHuman

		repos = append(repos, &sourceUser)
	}

	return
}
