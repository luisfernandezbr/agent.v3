package api

import (
	"errors"

	pjson "github.com/pinpt/go-common/json"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func UsersAll(qc QueryContext, resChan chan []sourcecode.User) error {
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, sub, err := UsersPage(qc, query)
		if err != nil {
			return pi, err
		}
		resChan <- sub
		return pi, nil
	})
}

type userGithub struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Email     string `json:"email"`
	Login     string `json:"login"`
}

func (s userGithub) Convert(customerID string, notInOrganization bool) (user sourcecode.User) {
	user.RefType = "sourcecode.User"
	user.CustomerID = customerID
	user.RefID = s.ID
	user.Name = s.Name
	user.AvatarURL = pstrings.Pointer(s.AvatarURL)
	user.Email = pstrings.Pointer(s.Email)
	user.Username = pstrings.Pointer(s.Login)
	// TODO: add to data model
	//user.NotInOrganization = notInOrganization
	return user
}

const userGithubFields = `{
	id
	name
	avatarUrl
	email
	login
}`

func UsersPage(qc QueryContext, queryParams string) (pi PageInfo, users []sourcecode.User, _ error) {
	qc.Logger.Debug("users request", "q", queryParams)

	query := `
	query {
		viewer {
			organization(login:"pinpt"){
				membersWithRole(` + queryParams + `) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
						hasPreviousPage
						startCursor
					}
					nodes ` + userGithubFields + `
				}
			}
		}
	}
	`

	var res struct {
		Data struct {
			Viewer struct {
				Organization struct {
					Members struct {
						TotalCount int          `json:"totalCount"`
						PageInfo   PageInfo     `json:"pageInfo"`
						Nodes      []userGithub `json:"nodes"`
					} `json:"membersWithRole"`
				} `json:"organization"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := qc.Request(query, &res)
	if err != nil {
		return pi, users, err
	}

	members := res.Data.Viewer.Organization.Members
	memberNodes := members.Nodes

	if len(memberNodes) == 0 {
		qc.Logger.Warn("no users found")
	}

	for _, data := range memberNodes {
		item := data.Convert(qc.CustomerID, false)
		users = append(users, item)
	}

	return members.PageInfo, users, nil
}

func User(qc QueryContext, login string) (user sourcecode.User, _ error) {
	qc.Logger.Debug("user request", "login", login)

	query := `
	query {
		user(login:` + pjson.Stringify(login) + `)` + userGithubFields + `
	}
	`

	var res struct {
		Data struct {
			User userGithub `json:"user"`
		} `json:"data"`
	}

	err := qc.Request(query, &res)
	if err != nil {
		return user, err
	}

	data := res.Data.User

	if data.ID == "" {
		panic("user not found for login: " + login)
		return user, errors.New("user not found for login: " + login)
	}

	return data.Convert(qc.CustomerID, true), nil
}
