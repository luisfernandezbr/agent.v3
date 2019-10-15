package main

import (
	"net/url"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
	"github.com/pinpt/agent.next/integrations/pkg/objsender2"
)

func (s *Integration) exportPullRequestsRepo(logger hclog.Logger, repo commonrepo.Repo, pullRequestSender *objsender2.Session, pullRequests chan []api.PullRequest, lastProcessed time.Time) error {

	err := api.PaginateNewerThan(logger, lastProcessed, func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestPage(s.qc, repo.ID, parameters, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}

		pullRequestSender.SetTotal(pi.Total)

		pullRequests <- res
		return pi, nil
	})
	if err != nil {
		return err
	}

	return nil
}
