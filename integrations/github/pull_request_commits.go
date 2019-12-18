package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/integration-sdk/sourcecode"
)

const pageSizeHeavyQueries = 50

func (s *Integration) exportPullRequestCommits(logger hclog.Logger, prID string) (res []*sourcecode.PullRequestCommit, _ error) {
	err := api.PaginateRegularWithPageSize(pageSizeHeavyQueries, func(query string) (api.PageInfo, error) {
		pi, sub, err := api.PullRequestCommitsPage(s.qc, prID, query)
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
