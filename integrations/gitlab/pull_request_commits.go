package main

import (
	"net/url"

	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
)

func (s *Integration) exportPullRequestCommits(logger hclog.Logger, repo commonrepo.Repo, prID string, prIID string) (res []*sourcecode.PullRequestCommit, _ error) {
	err := api.PaginateStartAt(s.logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		pi, sub, err := api.PullRequestCommitsPage(s.qc, repo, prID, prIID, paginationParams)
		if err != nil {
			return pi, err
		}
		for _, obj := range sub {
			res = append(res, obj)
		}
		return pi, nil
	})
	if err != nil {
		return nil, err
	}
	return
}
