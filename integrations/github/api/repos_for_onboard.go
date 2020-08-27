package api

import (
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/agent"

	pjson "github.com/pinpt/go-common/v10/json"
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

	type Repositories struct {
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
		qc.Logger.Warn("no repos found", "org", org.Login)
	}

	for _, data := range repoNodes {
		if data.UpdatedAt.Before(stopOnUpdatedAt) {
			return PageInfo{}, repos, nil
		}

		repo := &agent.RepoResponseRepos{}

		repoID := Repo{ID: data.ID, NameWithOwner: data.NameWithOwner}
		_, noPermissions, err := WebhookList(qc, repoID)
		if err != nil {
			repo.WebhookPermission = false
			qc.Logger.Error("could not list webhooks for repo", "err", err)
		} else {
			repo.WebhookPermission = !noPermissions
		}

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
