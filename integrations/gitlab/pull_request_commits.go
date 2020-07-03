package main

import (
	"net/url"

	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent/integrations/pkg/commonrepo"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/gitlab/api"
)

func (s *Integration) exportPullRequestCommits(logger hclog.Logger, repo commonrepo.Repo, pr api.PullRequest) (res []*sourcecode.PullRequestCommit, _ error) {
	err := api.PaginateStartAt(s.logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		pi, sub, err := api.PullRequestCommitsPage(s.qc, repo, pr, paginationParams)
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
