package main

import (
	"net/url"

	"github.com/pinpt/agent/integrations/pkg/commonrepo"

	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/bitbucket/api"
)

func (s *Integration) exportPullRequestCommits(logger hclog.Logger, repo commonrepo.Repo, prID string) (res []*sourcecode.PullRequestCommit, _ error) {
	err := api.Paginate(s.logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		pi, sub, err := api.PullRequestCommitsPage(s.qc, repo, prID, paginationParams)
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
