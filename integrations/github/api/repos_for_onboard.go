package api

import (
	"time"

	"github.com/pinpt/agent.next/pkg/ids"

	"github.com/pinpt/agent.next/pkg/date"
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
					name
					description
					primaryLanguage {
						name
					}			
					defaultBranchRef {
						target {
							... on Commit {
								oid
								url
								message
								author {
									name
									email
									avatarUrl
								}
								committedDate
								authoredDate
							}
						}
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
						Name            string    `json:"name"`
						Description     string    `json:"description"`
						PrimaryLanguage struct {
							Name string `json:"name"`
						} `json:"primaryLanguage"`
						DefaultBranchRef struct {
							Target struct {
								OID     string `json:"oid"`
								URL     string `json:"url"`
								Message string `json:"message"`
								Author  struct {
									Name      string `json:"name"`
									Email     string `json:"email"`
									AvatarURL string `json:"avatarUrl"`
								} `json:"author"`
								CommittedDate time.Time `json:"committedDate"`
								AuthoredDate  time.Time `json:"authoredDate"`
							} `json:"target"`
						} `json:"defaultBranchRef"`
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
		repoID := ids.CodeRepo(qc.CustomerID, qc.RefType, data.ID)

		repo.RefType = qc.RefType
		//repo.CustomerID = qc.CustomerID
		repo.RefID = data.ID
		repo.Name = data.Name
		repo.Description = data.Description
		repo.Language = data.PrimaryLanguage.Name

		lastCommitDate := data.DefaultBranchRef.Target.CommittedDate
		repo.Active = isActive(lastCommitDate, data.CreatedAt, data.IsFork, data.IsArchived)

		date.ConvertToModel(data.CreatedAt, &repo.CreatedDate)

		cdata := data.DefaultBranchRef.Target
		if cdata.OID != "" {
			commit := agent.RepoResponseReposLastCommit{}
			commit.CommitSha = cdata.OID
			commit.CommitID = ids.CodeCommit(qc.CustomerID, qc.RefType, repoID, commit.CommitSha)
			commit.URL = cdata.URL
			commit.Message = cdata.Message
			commit.Author.Name = cdata.Author.Name
			commit.Author.Email = cdata.Author.Email
			commit.Author.AvatarURL = cdata.Author.AvatarURL
			date.ConvertToModel(cdata.AuthoredDate, &commit.CreatedDate)
			repo.LastCommit = commit
		}

		repos = append(repos, repo)
	}

	return repositories.PageInfo, repos, nil
}

func isActive(lastCommitDate time.Time, createdAt time.Time, isFork bool, isArchived bool) bool {
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	oneMonthAgo := time.Now().AddDate(0, -1, 0)

	active := (lastCommitDate.After(sixMonthsAgo) ||
		createdAt.After(oneMonthAgo)) &&
		!isFork &&
		!isArchived

	return active
}
