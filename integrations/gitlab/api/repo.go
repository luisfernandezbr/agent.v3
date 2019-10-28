package api

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
	pstrings "github.com/pinpt/go-common/strings"
)

func ReposAll(qc interface{}, groupName string, res chan []commonrepo.Repo) error {
	return PaginateStartAt(qc.(QueryContext).Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
		pi, repos, err := ReposPageRESTAll(qc.(QueryContext), groupName, paginationParams)
		if err != nil {
			return pi, err
		}
		res <- repos
		return pi, nil
	})
}

func ReposPageRESTAll(qc QueryContext, groupName string, params url.Values) (page PageInfo, repos []commonrepo.Repo, err error) {
	qc.Logger.Debug("repos request")

	objectPath := pstrings.JoinURL("groups", groupName, "projects")

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

func GetSingleRepo(qc QueryContext, groupName string) (repoName string, err error) {
	qc.Logger.Debug("repos request")

	objectPath := pstrings.JoinURL("groups", groupName, "projects")

	var rr []struct {
		FullName string `json:"path_with_namespace"`
	}

	params := url.Values{}
	params.Set("per_page", "1")

	_, err = qc.Request(objectPath, params, &rr)
	if err != nil {
		return
	}

	for _, repo := range rr {
		repoName = repo.FullName
	}

	return
}
