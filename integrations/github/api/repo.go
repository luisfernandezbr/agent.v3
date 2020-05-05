package api

import (
	"fmt"
	"strings"
	"time"

	pjson "github.com/pinpt/go-common/json"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// Repo contains repo ID and name for passing around and logging
type Repo struct {
	ID            string
	NameWithOwner string
}

type RepoWithDefaultBranch struct {
	ID            string
	NameWithOwner string
	// DefaultBranch of the repo, could be empty if no commits yet. Used for getting commit_users
	DefaultBranch string
}

func (s RepoWithDefaultBranch) Repo() Repo {
	return Repo{ID: s.ID, NameWithOwner: s.NameWithOwner}
}

func ReposAll(qc QueryContext, org Org, res chan []RepoWithDefaultBranch) error {
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, sub, err := ReposPageInternal(qc, org, query)
		if err != nil {
			return pi, err
		}
		res <- sub
		return pi, nil
	})
}

func ReposAllSlice(qc QueryContext, org Org) (sl []RepoWithDefaultBranch, rerr error) {
	res := make(chan []RepoWithDefaultBranch)
	go func() {
		defer close(res)
		err := ReposAll(qc, org, res)
		if err != nil {
			rerr = err
		}
	}()
	for a := range res {
		for _, sub := range a {
			sl = append(sl, sub)
		}
	}
	return
}

func ReposPageInternal(qc QueryContext, org Org, queryParams string) (pi PageInfo, repos []RepoWithDefaultBranch, _ error) {

	var loginQuery string

	if org.Login == "" {
		loginQuery = "viewer{"
	} else {
		loginQuery = `organization(login:` + pjson.Stringify(org.Login) + `){`
	}

	query := `
	query {
		` + loginQuery + `
			repositories(` + queryParams + `) {
				totalCount
				pageInfo {
					hasNextPage
					endCursor
					hasPreviousPage
					startCursor
				}
				nodes {
					id
					nameWithOwner
					defaultBranchRef {
						name
					}
				}
			}
		}
	}
	`

	type Repositories struct {
		TotalCount int      `json:"totalCount"`
		PageInfo   PageInfo `json:"pageInfo"`
		Nodes      []struct {
			ID               string `json:"id"`
			NameWithOwner    string `json:"nameWithOwner"`
			DefaultBranchRef struct {
				Name string `json:"name"`
			} `json:"defaultBranchRef"`
		} `json:"nodes"`
	}

	var res struct {
		Data struct {
			Organization struct {
				Repositories *Repositories `json:"repositories"`
			} `json:"organization"`
			Viewer struct {
				Repositories *Repositories `json:"repositories"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := qc.Request(query, nil, &res)
	if err != nil {
		if strings.Contains(err.Error(), "Resource protected by organization SAML enforcement") {
			err = fmt.Errorf("The organization %s has SAML authentication enabled. You must grant your personal token access to your organization", org.Login)
		}
		return pi, repos, err
	}

	var repositories *Repositories

	if res.Data.Organization.Repositories != nil {
		repositories = res.Data.Organization.Repositories
	} else {
		repositories = res.Data.Viewer.Repositories
	}

	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		qc.Logger.Warn("no repos found")
		return pi, repos, nil
	}

	var batch []RepoWithDefaultBranch
	for _, data := range repoNodes {
		repo := RepoWithDefaultBranch{}
		repo.ID = data.ID
		repo.NameWithOwner = data.NameWithOwner
		repo.DefaultBranch = data.DefaultBranchRef.Name
		batch = append(batch, repo)
	}

	return repositories.PageInfo, batch, nil
}

func ReposPage(qc QueryContext, org Org, queryParams string, stopOnUpdatedAt time.Time) (pi PageInfo, repos []*sourcecode.Repo, totalCount int, rerr error) {
	qc.Logger.Debug("repos request", "q", queryParams)

	var loginQuery string

	if org.Login == "" {
		loginQuery = "viewer{"
	} else {
		loginQuery = `organization(login:` + pjson.Stringify(org.Login) + `){`
	}

	query := `
	query {
		` + loginQuery + `
			repositories(` + queryParams + `) {
				totalCount
				pageInfo {
					hasNextPage
					endCursor
					hasPreviousPage
					startCursor
				}
				nodes {
					updatedAt
					id
					nameWithOwner
					url	
					description		
					isArchived	
					primaryLanguage {
						name
					}
				}
			}
		}
	}
	`

	type Repositories struct {
		TotalCount int      `json:"totalCount"`
		PageInfo   PageInfo `json:"pageInfo"`
		Nodes      []struct {
			UpdatedAt       time.Time `json:"updatedAt"`
			ID              string    `json:"id"`
			NameWithOwner   string    `json:"nameWithOwner"`
			URL             string    `json:"url"`
			Description     string    `json:"description"`
			IsArchived      bool      `json:"isArchived"`
			PrimaryLanguage struct {
				Name string `json:"name"`
			} `json:"primaryLanguage"`
		} `json:"nodes"`
	}

	var res struct {
		Data struct {
			Organization struct {
				Repositories *Repositories `json:"repositories"`
			} `json:"organization"`
			Viewer struct {
				Repositories *Repositories `json:"repositories"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := qc.Request(query, nil, &res)
	if err != nil {
		rerr = err
		return
	}

	var repositories *Repositories

	if res.Data.Organization.Repositories != nil {
		repositories = res.Data.Organization.Repositories
	} else {
		repositories = res.Data.Viewer.Repositories
	}

	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		qc.Logger.Warn("no repos found")
	}

	for _, data := range repoNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			return
		}
		if data.ID == "" || data.NameWithOwner == "" || data.URL == "" {
			rerr = fmt.Errorf("missing required data for repo %+v", data)
			return
		}
		repo := &sourcecode.Repo{}
		repo.RefType = "github"
		repo.CustomerID = qc.CustomerID
		repo.RefID = data.ID
		repo.Name = data.NameWithOwner
		repo.URL = data.URL
		repo.Language = data.PrimaryLanguage.Name
		repo.Description = data.Description
		repo.Active = !data.IsArchived
		repos = append(repos, repo)
	}

	return repositories.PageInfo, repos, repositories.TotalCount, nil
}
