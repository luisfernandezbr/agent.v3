package api

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/go-common/datetime"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func AvatarURL(qc QueryContext, email string) (string, error) {
	qc.Logger.Debug("avatar url", "email", email)

	objectPath := pstrings.JoinURL("avatar")

	params := url.Values{}
	params.Set("email", email)

	var avatarResponse map[string]string

	if _, err := qc.Request(objectPath, params, &avatarResponse); err != nil {
		return "", err
	}

	return avatarResponse["avatar_url"], nil
}

func RepoLastCommit(qc QueryContext, repo *agent.RepoResponseRepos) (lastCommit agent.RepoResponseReposLastCommit, err error) {
	qc.Logger.Debug("last commit request", "repo", repo.Name)

	objectPath := pstrings.JoinURL("projects", repo.RefID, "repository", "commits")
	params := url.Values{}
	params.Set("page", "1")
	params.Set("per_page", "1")

	var commits []struct {
		ID          string    `json:"id"`
		Message     string    `json:"message"`
		CreatedAt   time.Time `json:"created_at"`
		AuthorName  string    `json:"author_name"`
		AuthorEmail string    `json:"author_email"`
	}

	_, err = qc.Request(objectPath, params, &commits)
	if err != nil {
		return lastCommit, err
	}

	if len(commits) == 0 {
		return lastCommit, nil
	}

	lastCommitSource := commits[0]
	url, err := url.Parse(qc.BaseURL)
	if err != nil {
		return lastCommit, err
	}
	lastCommit = agent.RepoResponseReposLastCommit{
		Message:   lastCommitSource.Message,
		URL:       url.Scheme + "://" + url.Hostname() + "/" + repo.Name + "/commit/" + lastCommitSource.ID,
		CommitSha: lastCommitSource.ID,
		CommitID:  ids.CodeCommit(qc.CustomerID, qc.RefType, repo.RefID, lastCommitSource.ID),
	}

	authorLastCommit := agent.RepoResponseReposLastCommitAuthor{
		Name:  lastCommitSource.AuthorName,
		Email: lastCommitSource.AuthorEmail,
	}

	lastCommit.Author = authorLastCommit

	date.ConvertToModel(lastCommitSource.CreatedAt, &lastCommit.CreatedDate)

	return
}

func RepoLanguage(qc QueryContext, repoID string) (string, error) {
	qc.Logger.Debug("language request", "repo", repoID)

	objectPath := pstrings.JoinURL("projects", repoID, "languages")

	var languages map[string]float32

	_, err := qc.Request(objectPath, nil, &languages)
	if err != nil {
		return "", err
	}

	var maxValue float32
	var maxLanguage string
	for language, percentage := range languages {
		if percentage > maxValue {
			maxValue = percentage
			maxLanguage = language
		}
	}

	return maxLanguage, nil
}

func ReposOnboardPage(qc QueryContext, params url.Values) (page PageInfo, repos []*agent.RepoResponseRepos, err error) {
	qc.Logger.Debug("repos request")

	objectPath := "projects"

	params.Set("membership", "true")
	params.Set("per_page", "100")

	var rr []struct {
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   string    `json:"last_activity_at"`
		ID          int64     `json:"id"`
		FullName    string    `json:"path_with_namespace,omitempty"`
		Description string    `json:"description,omitempty"`
	}

	page, err = qc.Request(objectPath, params, &rr)
	if err != nil {
		return
	}

	for _, v := range rr {
		ID := fmt.Sprint(v.ID)
		repo := &agent.RepoResponseRepos{
			RefID:       ID,
			RefType:     "gitlab",
			Name:        v.FullName,
			Description: v.Description,
			Active:      true,
		}

		repo.LastCommit, err = RepoLastCommit(qc, repo)
		if err != nil {
			return
		}

		repo.Language, err = RepoLanguage(qc, ID)
		if err != nil {
			return
		}

		date.ConvertToModel(v.CreatedAt, &repo.CreatedDate)

		repos = append(repos, repo)
	}

	return
}

func ReposOnboardPageGraphQL(qc QueryContext, groupName, pageSize, after string) (afterCursor string, repos []*agent.RepoResponseRepos, err error) {
	qc.Logger.Debug("repos request")

	var afterParam string

	firstParam := "first:" + pageSize
	if after != "" {
		afterParam = ",after:\"" + after + "\""
	}

	projectParams := firstParam + afterParam

	query := `
		group(fullPath:"` + groupName + `"){
			projects(` + projectParams + `){
				edges{
					cursor
					node{
						id
						fullPath
						description
						createdAt
						repository{
							tree{
								lastCommit{
									author{
										avatarUrl
									}
								}
							}
						}
					}
				}
			}
		}
	`

	var res struct {
		Data struct {
			Group struct {
				Projects struct {
					Edges []struct {
						Cursor string `json:"cursor"`
						Node   struct {
							ID          string    `json:"id"`
							FullName    string    `json:"nameWithNamespace"`
							Description string    `json:"description"`
							CreatedAt   time.Time `json:"createdAt"`
							Repository  struct {
								Tree struct {
									LastCommit struct {
										Author struct {
											AvatarURL string `json:"avatarUrl"`
										} `json:"author"`
									} `json:"lastCommit"`
								} `json:"tree"`
							} `json:"repository"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"projects"`
			} `json:"group"`
		} `json:"data"`
	}

	err = qc.RequestGraphQL(query, &res)
	if err != nil {
		return
	}

	edges := res.Data.Group.Projects.Edges

	for _, edge := range edges {
		r := edge.Node
		afterCursor = edge.Cursor
		ID := getRepoID(r.ID)
		repo := &agent.RepoResponseRepos{
			RefID:       ID,
			RefType:     qc.RefType,
			Name:        r.FullName,
			Description: r.Description,
			Active:      true,
		}

		repo.LastCommit, err = RepoLastCommit(qc, repo)
		if err != nil {
			return
		}

		repo.LastCommit.Author.AvatarURL = r.Repository.Tree.LastCommit.Author.AvatarURL

		repo.Language, err = RepoLanguage(qc, ID)
		if err != nil {
			return
		}

		date.ConvertToModel(r.CreatedAt, &repo.CreatedDate)

		repos = append(repos, repo)
	}

	return
}

func ReposPageREST(qc QueryContext, groupID string, params url.Values, stopOnUpdatedAt time.Time) (page PageInfo, repos []*sourcecode.Repo, err error) {
	qc.Logger.Debug("repos request", "group", groupID)

	objectPath := pstrings.JoinURL("groups", groupID, "projects")

	var rr []struct {
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"last_activity_at"`
		ID          int64     `json:"id"`
		FullName    string    `json:"path_with_namespace,omitempty"`
		Description string    `json:"description,omitempty"`
		WebURL      string    `json:"web_url,omitempty"`
	}

	page, err = qc.Request(objectPath, params, &rr)
	if err != nil {
		return
	}

	for _, repo := range rr {
		if repo.UpdatedAt.Before(stopOnUpdatedAt) {
			return
		}
		id := strconv.FormatInt(repo.ID, 10)
		repo := &sourcecode.Repo{
			RefID:       id,
			RefType:     qc.RefType,
			CustomerID:  qc.CustomerID,
			Name:        repo.FullName,
			URL:         repo.WebURL,
			Description: repo.Description,
			UpdatedAt:   datetime.TimeToEpoch(repo.UpdatedAt),
			Active:      true,
		}

		repo.Language, err = RepoLanguage(qc, id)
		if err != nil {
			return
		}

		repos = append(repos, repo)
	}

	return
}

func getRepoID(gID string) string {
	tokens := strings.Split(gID, "/")
	return tokens[len(tokens)-1]
}

type PaginateStartAtFn func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error)

func PaginateStartAt(log hclog.Logger, fn PaginateStartAtFn) error {
	pageOffset := 0
	nextPage := "1"
	for {
		q := url.Values{}
		q.Add("page", nextPage)
		pageInfo, err := fn(log, q)
		if err != nil {
			return err
		}
		if pageInfo.Page == pageInfo.TotalPages {
			return nil
		}
		if pageInfo.PageSize == 0 {
			return errors.New("pageSize is 0")
		}

		nextPage = pageInfo.NextPage
		pageOffset += pageInfo.PageSize
	}
}

type PaginateGraphQLStartAtFn func(log hclog.Logger, pageSize string, after string) (afterCursor string, _ error)

func PaginateGraphQL(log hclog.Logger, fn PaginateGraphQLStartAtFn) error {
	var previousAfterCursor string
	for {
		afterCursor, err := fn(log, "100", previousAfterCursor)
		if err != nil {
			return err
		}
		if afterCursor == "" {
			break
		}
		previousAfterCursor = afterCursor
	}

	return nil
}

type PaginateNewerThanFn func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (PageInfo, error)

func PaginateNewerThan(log hclog.Logger, lastProcessed time.Time, fn PaginateNewerThanFn) error {
	pageOffset := 0
	nextPage := "1"

	if lastProcessed.IsZero() {
		for {
			p := url.Values{}
			p.Add("page", nextPage)
			pageInfo, err := fn(log, p, lastProcessed)
			if err != nil {
				return err
			}
			if pageInfo.Page == pageInfo.TotalPages {
				return nil
			}
			if pageInfo.PageSize == 0 {
				return errors.New("pageSize is 0")
			}
			nextPage = pageInfo.NextPage
			pageOffset += pageInfo.PageSize
		}
	}

	for {
		p := url.Values{}
		p.Add("page", nextPage)
		p.Add("order_by", "last_activity_at")
		pageInfo, err := fn(log, p, lastProcessed)
		if err != nil {
			return err
		}
		if pageInfo.Page == pageInfo.TotalPages {
			return nil
		}
		if pageInfo.PageSize == 0 {
			return errors.New("pageSize is 0")
		}
		nextPage = pageInfo.NextPage
		pageOffset += pageInfo.PageSize
	}
}
