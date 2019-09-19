package api

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/commonrepo"
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
