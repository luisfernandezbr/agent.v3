package api

import (
	"errors"

	pjson "github.com/pinpt/go-common/json"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type User struct {
	Typename  string `json:"__typename"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Login     string `json:"login"`
	URL       string `json:"url"`
}

const userFields = `{
	__typename
	... on User {
		id
		name
	}
	... on Bot {
		id
	}
	avatarUrl
	login
	url
}`

type userGithub struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Login     string `json:"login"`
	URL       string `json:"url"`
}

const userGithubFields = `{
	id
	name
	avatarUrl
	login
	url
}`

func (s userGithub) Convert(customerID string, orgMember bool) (user *sourcecode.User) {
	user = &sourcecode.User{}
	user.RefType = "github"
	user.CustomerID = customerID
	user.RefID = s.ID
	user.Name = s.Name
	user.AvatarURL = pstrings.Pointer(s.AvatarURL)
	user.Username = pstrings.Pointer(s.Login)
	user.Member = orgMember
	user.Type = sourcecode.UserTypeHuman
	user.URL = pstrings.Pointer(s.URL)
	return user
}

func GetUser(qc QueryContext, login string, orgMember bool) (
	user *sourcecode.User, _ error) {

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

	err := qc.Request(query, nil, &res)
	if err != nil {
		return user, err
	}

	data := res.Data.User

	if data.ID == "" {
		return user, errors.New("user not found for login: " + login)
	}

	return data.Convert(qc.CustomerID, orgMember), nil
}

func UsersAll(qc QueryContext, org Org, resChan chan []*sourcecode.User) error {
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, sub, err := UsersPage(qc, org, query)
		if err != nil {
			return pi, err
		}
		resChan <- sub
		return pi, nil
	})
}

func UsersPage(qc QueryContext, org Org, queryParams string) (pi PageInfo, users []*sourcecode.User, _ error) {
	qc.Logger.Debug("users request", "q", queryParams)

	query := `
	query {
		viewer {
			organization(login:` + pjson.Stringify(org.Login) + `){
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

	err := qc.Request(query, nil, &res)
	if err != nil {
		return pi, users, err
	}

	members := res.Data.Viewer.Organization.Members
	memberNodes := members.Nodes

	if len(memberNodes) == 0 {
		qc.Logger.Warn("no users found")
	}

	for _, data := range memberNodes {
		item := data.Convert(qc.CustomerID, true)
		users = append(users, item)
	}

	return members.PageInfo, users, nil
}

func UsersEnterpriseAll(qc QueryContext, resChan chan []*sourcecode.User) error {
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, sub, err := UsersEnterprisePage(qc, query)
		if err != nil {
			return pi, err
		}
		resChan <- sub
		return pi, nil
	})
}

func UsersEnterprisePage(qc QueryContext, queryParams string) (pi PageInfo, users []*sourcecode.User, _ error) {
	qc.Logger.Debug("users request", "q", queryParams)

	query := `
	query {
		users(` + queryParams + `) {
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
	`

	var res struct {
		Data struct {
			Users struct {
				TotalCount int          `json:"totalCount"`
				PageInfo   PageInfo     `json:"pageInfo"`
				Nodes      []userGithub `json:"nodes"`
			} `json:"users"`
		} `json:"data"`
	}

	err := qc.Request(query, nil, &res)
	if err != nil {
		return pi, users, err
	}

	members := res.Data.Users
	memberNodes := members.Nodes

	if len(memberNodes) == 0 {
		qc.Logger.Warn("no users found")
	}

	for _, data := range memberNodes {
		item := data.Convert(qc.CustomerID, true)
		users = append(users, item)
	}

	return members.PageInfo, users, nil
}
