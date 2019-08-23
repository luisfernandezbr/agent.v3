package api

import (
	"time"

	pjson "github.com/pinpt/go-common/json"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// Repo contains the data needed for exporting other resources depending on it
type Repo struct {
	ID            string
	NameWithOwner string
	// DefaultBranch of the repo, could be empty if no commits yet. Used for getting commit_users
	DefaultBranch string
}

func ReposAll(qc QueryContext, org Org, res chan []Repo) error {
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, sub, err := ReposPageInternal(qc, org, query)
		if err != nil {
			return pi, err
		}
		res <- sub
		return pi, nil
	})
}

func ReposAllSlice(qc QueryContext, org Org) (sl []Repo, rerr error) {
	res := make(chan []Repo)
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

func ReposPageInternal(qc QueryContext, org Org, queryParams string) (pi PageInfo, repos []Repo, _ error) {

	query := `
	query {
		organization(login:` + pjson.Stringify(org.Login) + `){
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

	var res struct {
		Data struct {
			Organization struct {
				Repositories struct {
					TotalCount int      `json:"totalCount"`
					PageInfo   PageInfo `json:"pageInfo"`
					Nodes      []struct {
						ID               string `json:"id"`
						NameWithOwner    string `json:"nameWithOwner"`
						DefaultBranchRef struct {
							Name string `json:"name"`
						} `json:"defaultBranchRef"`
					} `json:"nodes"`
				} `json:"repositories"`
			} `json:"organization"`
		} `json:"data"`
	}

	err := qc.Request(query, &res)
	if err != nil {
		return pi, repos, err
	}

	repositories := res.Data.Organization.Repositories
	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		qc.Logger.Warn("no repos found")
		return pi, repos, nil
	}

	var batch []Repo
	for _, data := range repoNodes {
		repo := Repo{}
		repo.ID = data.ID
		repo.NameWithOwner = data.NameWithOwner
		repo.DefaultBranch = data.DefaultBranchRef.Name
		batch = append(batch, repo)
	}

	return repositories.PageInfo, batch, nil
}

func ReposPage(qc QueryContext, org Org, queryParams string, stopOnUpdatedAt time.Time) (pi PageInfo, repos []*sourcecode.Repo, _ error) {
	qc.Logger.Debug("repos request", "q", queryParams)

	query := `
	query {
		organization(login:` + pjson.Stringify(org.Login) + `){
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
					name
					url						
				}
			}
		}
	}
	`

	var res struct {
		Data struct {
			Organization struct {
				Repositories struct {
					TotalCount int      `json:"totalCount"`
					PageInfo   PageInfo `json:"pageInfo"`
					Nodes      []struct {
						UpdatedAt     time.Time `json:"updatedAt"`
						ID            string    `json:"id"`
						NameWithOwner string    `json:"nameWithOwner"`
						URL           string    `json:"url"`
					} `json:"nodes"`
				} `json:"repositories"`
			} `json:"organization"`
		} `json:"data"`
	}

	err := qc.Request(query, &res)
	if err != nil {
		return pi, repos, err
	}

	repositories := res.Data.Organization.Repositories
	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		qc.Logger.Warn("no repos found")
	}

	for _, data := range repoNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			return PageInfo{}, repos, nil
		}
		repo := &sourcecode.Repo{}
		repo.RefType = "sourcecode.Repo"
		repo.CustomerID = qc.CustomerID
		repo.RefID = data.ID
		repo.Name = data.NameWithOwner
		repo.URL = data.URL
		repos = append(repos, repo)
	}

	return repositories.PageInfo, repos, nil
}
