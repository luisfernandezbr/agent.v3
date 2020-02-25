package api

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/pinpt/agent/integrations/pkg/commonrepo"

	"github.com/pinpt/integration-sdk/sourcecode"

	pstrings "github.com/pinpt/go-common/strings"
)

type User struct {
	ID        int64
	Email     string
	Username  string
	Name      string
	AvatarURL string
	URL       string
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
		WebURL    string `json:"web_url"`
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
			URL:       user.WebURL,
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

func RepoUsersPageREST(qc QueryContext, repo commonrepo.Repo, params url.Values) (page PageInfo, repos []*sourcecode.User, err error) {
	qc.Logger.Debug("users request", "repo", repo)

	objectPath := pstrings.JoinURL("projects", repo.ID, "users")

	var ru []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
		WebURL    string `json:"web_url"`
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
		sourceUser.URL = pstrings.Pointer(user.WebURL)

		repos = append(repos, &sourceUser)
	}

	return
}
