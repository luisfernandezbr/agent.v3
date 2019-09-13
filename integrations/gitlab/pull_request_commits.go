package main

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
)

func (s *Integration) exportPullRequestCommits(logger hclog.Logger, repoID string, prID string) (res []string, _ error) {
	err := api.PaginateStartAt(s.logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		pi, sub, err := api.PullRequestCommitsPage(s.qc, repoID, prID, paginationParams)
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
