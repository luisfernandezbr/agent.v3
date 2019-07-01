package api

import (
	"time"

	"github.com/pinpt/go-datamodel/sourcecode"
)

// Repo contains the data needed for exporting other resources depending on it
type Repo struct {
	ID   string
	Name string
	// DefaultBranch of the repo, could be empty if no commits yet. Used for getting commit_users
	DefaultBranch string
}

func ReposAll(qc QueryContext, res chan []Repo) error {
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, sub, err := ReposPageInternal(qc, query)
		if err != nil {
			return pi, err
		}
		res <- sub
		return pi, nil
	})
}

func ReposAllSlice(qc QueryContext) ([]Repo, error) {
	res := make(chan []Repo)
	go func() {
		defer close(res)
		err := ReposAll(qc, res)
		if err != nil {
			panic(err)
		}
	}()
	var sl []Repo
	for a := range res {
		for _, sub := range a {
			sl = append(sl, sub)
		}
	}
	return sl, nil
}

func ReposPageInternal(qc QueryContext, queryParams string) (pi PageInfo, repos []Repo, _ error) {

	query := `
	query {
		viewer {
			organization(login:"pinpt"){
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
						name
						defaultBranchRef {
							name
						}
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
					Repositories struct {
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes      []struct {
							ID               string `json:"id"`
							Name             string `json:"name"`
							DefaultBranchRef struct {
								Name string `json:"name"`
							} `json:"defaultBranchRef"`
						} `json:"nodes"`
					} `json:"repositories"`
				} `json:"organization"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := qc.Request(query, &res)
	if err != nil {
		return pi, repos, err
	}

	repositories := res.Data.Viewer.Organization.Repositories
	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		qc.Logger.Warn("no repos found")
		return pi, repos, nil
	}

	var batch []Repo
	for _, data := range repoNodes {
		repo := Repo{}
		repo.ID = data.ID
		repo.Name = data.Name
		repo.DefaultBranch = data.DefaultBranchRef.Name
		batch = append(batch, repo)
	}

	return repositories.PageInfo, batch, nil
}

func ReposPage(qc QueryContext, queryParams string, stopOnUpdatedAt time.Time) (pi PageInfo, repos []sourcecode.Repo, _ error) {
	qc.Logger.Debug("repos request", "q", queryParams)

	query := `
	query {
		viewer {
			organization(login:"pinpt"){
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
	}
	`

	var res struct {
		Data struct {
			Viewer struct {
				Organization struct {
					Repositories struct {
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes      []struct {
							UpdatedAt time.Time `json:"updatedAt"`
							ID        string    `json:"id"`
							Name      string    `json:"name"`
							URL       string    `json:"url"`
						} `json:"nodes"`
					} `json:"repositories"`
				} `json:"organization"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := qc.Request(query, &res)
	if err != nil {
		return pi, repos, err
	}

	repositories := res.Data.Viewer.Organization.Repositories
	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		qc.Logger.Warn("no repos found")
	}

	for _, data := range repoNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			return PageInfo{}, repos, nil
		}
		repo := sourcecode.Repo{}
		repo.RefType = "sourcecode.Repo"
		repo.CustomerID = qc.CustomerID
		repo.RefID = data.ID
		repo.Name = data.Name
		repo.URL = data.URL
		repos = append(repos, repo)
	}

	return repositories.PageInfo, repos, nil
}
