package main

import (
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/integrations/pkg/objsender"
)

func (s *Integration) exportPullRequestsRepo(logger hclog.Logger, repo api.Repo, pullRequestSender *objsender.Session, pullRequests chan []api.PullRequest, lastProcessed time.Time) error {
	return api.PaginateNewerThan(lastProcessed, func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, totalCount, err := api.PullRequestsPage(s.qc, repo.ID, query, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		err = pullRequestSender.SetTotal(totalCount)
		if err != nil {
			return pi, err
		}
		pullRequests <- res
		return pi, nil
	})
}
