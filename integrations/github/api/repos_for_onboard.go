package api

import (
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/agent"

	pjson "github.com/pinpt/go-common/json"
)

func ReposForOnboardAll(qc QueryContext, org Org) (res []*agent.RepoResponseRepos, _ error) {
	err := PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, sub, err := ReposForOnboardPage(qc, org, query, time.Time{})
		if err != nil {
			return pi, err
		}
		for _, r := range sub {
			res = append(res, r)
		}
		return pi, nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func ReposForOnboardPage(qc QueryContext, org Org, queryParams string, stopOnUpdatedAt time.Time) (pi PageInfo, repos []*agent.RepoResponseRepos, _ error) {
	qc.Logger.Debug("repos request", "q", queryParams, "org", org.Login)

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
					createdAt
					updatedAt
					id
					nameWithOwner
					description
					primaryLanguage {
						name
					}			
					isFork
					isArchived
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
						CreatedAt       time.Time `json:"createdAt"`
						UpdatedAt       time.Time `json:"updatedAt"`
						ID              string    `json:"id"`
						NameWithOwner   string    `json:"nameWithOwner"`
						Description     string    `json:"description"`
						PrimaryLanguage struct {
							Name string `json:"name"`
						} `json:"primaryLanguage"`
						IsFork     bool `json:"isFork"`
						IsArchived bool `json:"isArchived"`
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
		qc.Logger.Warn("no repos found", "org", org.Login)
	}

	for _, data := range repoNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			return PageInfo{}, repos, nil
		}

		repo := &agent.RepoResponseRepos{}
		repo.RefType = qc.RefType
		repo.RefID = data.ID
		repo.Name = data.NameWithOwner
		repo.Description = data.Description
		repo.Language = data.PrimaryLanguage.Name
		repo.Active = !data.IsFork && !data.IsArchived

		date.ConvertToModel(data.CreatedAt, &repo.CreatedDate)

		repos = append(repos, repo)
	}

	return repositories.PageInfo, repos, nil
}
