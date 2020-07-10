package main

import (
	"net/url"
	"time"

	"github.com/pinpt/agent/integrations/bitbucket/api"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/integrations/pkg/objsender"

	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/hashicorp/go-hclog"
)

func (s *Integration) exportPullRequestCommits(logger hclog.Logger, repo commonrepo.Repo, pr sourcecode.PullRequest, prCommitsSender *objsender.Session) (res []*sourcecode.PullRequestCommit, _ error) {
	err := api.PaginateNewerThan(s.logger, prCommitsSender.LastProcessedTime(), func(log hclog.Logger, paginationParams url.Values, stopOnUpdatedAt time.Time) (page api.PageInfo, _ error) {
		pi, sub, err := api.PullRequestCommitsPage(s.qc, repo, pr, paginationParams, stopOnUpdatedAt)
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
