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

	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// ReposOnboardPage get repositories page for onboard
func ReposOnboardPage(qc QueryContext, groupName string, params url.Values) (page PageInfo, repos []*agent.RepoResponseRepos, err error) {
	qc.Logger.Debug("repos request", "group", groupName)

	objectPath := pstrings.JoinURL("groups", url.QueryEscape(groupName), "projects")

	params.Set("membership", "true")
	params.Set("per_page", "100")

	var rr []struct {
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   string    `json:"last_activity_at"`
		ID          int64     `json:"id"`
		FullName    string    `json:"path_with_namespace"`
		Description string    `json:"description"`
	}

	page, err = qc.Request(objectPath, params, &rr)
	if err != nil {
		return
	}

	for _, v := range rr {
		ID := fmt.Sprint(v.ID)
		repo := &agent.RepoResponseRepos{
			RefID:       ID,
			RefType:     qc.RefType,
			Name:        v.FullName,
			Description: v.Description,
			Active:      true,
		}

		repo.LastCommit, err = repoLastCommit(qc, repo)
		if err != nil {
			return
		}

		repo.Language, err = repoLanguage(qc, ID)
		if err != nil {
			return
		}

		date.ConvertToModel(v.CreatedAt, &repo.CreatedDate)

		repos = append(repos, repo)
	}

	return
}

// ReposPage get repositories page after stopOnUpdatedAt
func ReposPage(qc QueryContext, groupName string, params url.Values, stopOnUpdatedAt time.Time) (page PageInfo, repos []*sourcecode.Repo, err error) {
	qc.Logger.Debug("repos request", "group", groupName)

	objectPath := pstrings.JoinURL("groups", url.QueryEscape(groupName), "projects")

	var rr []struct {
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"last_activity_at"`
		ID          int64     `json:"id"`
		FullName    string    `json:"path_with_namespace"`
		Description string    `json:"description"`
		WebURL      string    `json:"web_url"`
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

		repo.Language, err = repoLanguage(qc, id)
		if err != nil {
			return
		}

		repos = append(repos, repo)
	}

	return
}

// ReposAll get all group repos available
func ReposAll(qc interface{}, groupName string, res chan []commonrepo.Repo) error {
	return PaginateStartAt(qc.(QueryContext).Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
		pi, repos, err := ReposPageCommon(qc.(QueryContext), groupName, paginationParams)
		if err != nil {
			return pi, err
		}
		res <- repos
		return pi, nil
	})
}

// ReposPageCommon get common info repos page
func ReposPageCommon(qc QueryContext, groupName string, params url.Values) (page PageInfo, repos []commonrepo.Repo, err error) {
	qc.Logger.Debug("repos request")

	objectPath := pstrings.JoinURL("groups", url.QueryEscape(groupName), "projects")

	var rr []struct {
		ID            int64  `json:"id"`
		FullName      string `json:"path_with_namespace"`
		DefaultBranch string `json:"default_branch"`
	}

	page, err = qc.Request(objectPath, params, &rr)
	if err != nil {
		return
	}

	for _, repo := range rr {
		repo := commonrepo.Repo{
			ID:            fmt.Sprint(repo.ID),
			NameWithOwner: repo.FullName,
			DefaultBranch: repo.DefaultBranch,
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

type PaginateNewerThanFn func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (PageInfo, error)

func PaginateNewerThan(log hclog.Logger, lastProcessed time.Time, fn PaginateNewerThanFn) error {
	pageOffset := 0
	nextPage := "1"
	p := url.Values{}
	p.Set("per_page", "100")

	if lastProcessed.IsZero() {
		for {
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

func repoLastCommit(qc QueryContext, repo *agent.RepoResponseRepos) (lastCommit agent.RepoResponseReposLastCommit, err error) {
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

func repoLanguage(qc QueryContext, repoID string) (string, error) {
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
