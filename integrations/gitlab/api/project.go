package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/go-common/v10/datetime"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/pkg/date"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// ReposOnboardPage get repositories page for onboard
func ReposOnboardPage(qc QueryContext, group *Group, params url.Values) (page PageInfo, repos []*agent.RepoResponseRepos, err error) {

	params.Set("membership", "true")
	params.Set("per_page", "100")
	params.Set("with_shared", "no")

	qc.Logger.Debug("repos request", "group", group.FullPath, "group_id", group.ID, "params", params)

	objectPath := pstrings.JoinURL("groups", group.ID, "projects")

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
		ID := strconv.FormatInt(v.ID, 10)
		repo := &agent.RepoResponseRepos{
			RefID:       ID,
			RefType:     qc.RefType,
			Name:        v.FullName,
			Description: v.Description,
			Active:      true,
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
func ReposPage(qc QueryContext, group *Group, params url.Values) (page PageInfo, repos []*sourcecode.Repo, err error) {

	params.Set("with_shared", "no")

	qc.Logger.Debug("repos request", "group", group.FullPath, "group_id", group.ID, "params", params)

	objectPath := pstrings.JoinURL("groups", group.ID, "projects")

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
		refID := strconv.FormatInt(repo.ID, 10)
		repo := &sourcecode.Repo{
			RefID:       refID,
			RefType:     qc.RefType,
			CustomerID:  qc.CustomerID,
			Name:        repo.FullName,
			URL:         repo.WebURL,
			Description: repo.Description,
			UpdatedAt:   datetime.TimeToEpoch(repo.UpdatedAt),
			Active:      true,
		}

		repo.Language, err = repoLanguage(qc, refID)
		if err != nil {
			return
		}

		repos = append(repos, repo)
	}

	return
}

// ReposAll get all group repos available
func ReposAll(qc interface{}, group *Group, res chan []commonrepo.Repo) error {
	return PaginateStartAt(qc.(QueryContext).Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
		pi, repos, err := ReposPageCommon(qc.(QueryContext), group, paginationParams)
		if err != nil {
			return pi, err
		}
		res <- repos
		return pi, nil
	})
}

// ReposPageCommon get common info repos page
func ReposPageCommon(qc QueryContext, group *Group, params url.Values) (page PageInfo, repos []commonrepo.Repo, err error) {

	params.Set("with_shared", "no")

	qc.Logger.Debug("repos request", "group", group.FullPath, "group_id", group.ID, "params", params)

	objectPath := pstrings.JoinURL("groups", group.ID, "projects")

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
			RefID:         fmt.Sprint(repo.ID),
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

func repoLanguage(qc QueryContext, repoID string) (maxLanguage string, err error) {
	qc.Logger.Debug("language request", "repo", repoID)

	objectPath := pstrings.JoinURL("projects", repoID, "languages")

	var languages map[string]float32

	if _, err = qc.Request(objectPath, nil, &languages); err != nil {
		return "", err
	}

	var maxValue float32
	for language, percentage := range languages {
		if percentage > maxValue {
			maxValue = percentage
			maxLanguage = language
		}
	}

	return maxLanguage, nil
}
