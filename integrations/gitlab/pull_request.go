package main

import (
	"net/url"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
)

func (s *Integration) exportPullRequestsRepo(logger hclog.Logger, repo api.Repo, pullRequests chan []api.PullRequest, lastProcessed time.Time) error {
	return api.PaginateNewerThan(logger, lastProcessed, func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestPage(s.qc, repo.ID, parameters, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		pullRequests <- res
		return pi, nil
	})
}
