package api

import (
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func UsersAll(qc QueryContext, resChan chan []sourcecode.User) error {
	defer close(resChan)
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, sub, err := UsersPage(qc, query)
		if err != nil {
			return pi, err
		}
		resChan <- sub
		return pi, nil
	})
}

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
					nodes {
						id
						name
						avatarUrl
						email
						login
					}
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
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes      []struct {
							ID        string `json:"id"`
							Name      string `json:"name"`
							AvatarURL string `json:"avatarUrl"`
							Email     string `json:"email"`
							Login     string `json:"login"`
						} `json:"nodes"`
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
		item := sourcecode.User{}
		item.RefType = "sourcecode.User"
		item.CustomerID = qc.CustomerID
		item.RefID = data.ID
		item.Name = data.Name
		item.AvatarURL = pstrings.Pointer(data.AvatarURL)
		item.Email = pstrings.Pointer(data.Email)
		item.Username = pstrings.Pointer(data.Login)
		users = append(users, item)
	}

	return members.PageInfo, users, nil
}
