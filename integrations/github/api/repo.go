package api

import (
	"time"

	"github.com/pinpt/go-datamodel/sourcecode"
)

func ReposAllIDs(qc QueryContext, idChan chan []string) error {
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, ids, err := ReposPageIDs(qc, query)
		if err != nil {
			return pi, err
		}
		idChan <- ids
		return pi, nil
	})
}

func ReposAllIDsSlice(qc QueryContext) ([]string, error) {
	res := make(chan []string)
	go func() {
		defer close(res)
		err := ReposAllIDs(qc, res)
		if err != nil {
			panic(err)
		}
	}()
	var sl []string
	for a := range res {
		for _, id := range a {
			sl = append(sl, id)
		}
	}
	return sl, nil
}

func ReposPageIDs(qc QueryContext, queryParams string) (pi PageInfo, ids IDs, _ error) {

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
							ID string `json:"id"`
						} `json:"nodes"`
					} `json:"repositories"`
				} `json:"organization"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := qc.Request(query, &res)
	if err != nil {
		return pi, ids, err
	}

	repositories := res.Data.Viewer.Organization.Repositories
	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		qc.Logger.Warn("no repos found")
		return pi, ids, nil
	}

	var batch IDs
	for _, data := range repoNodes {
		batch = append(batch, data.ID)
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
