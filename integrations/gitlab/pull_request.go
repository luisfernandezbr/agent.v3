package main

import (
	"net/url"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/gitlab/api"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/integrations/pkg/objsender"
)

func (s *Integration) exportPullRequestsRepo(logger hclog.Logger, repo commonrepo.Repo, pullRequestSender *objsender.Session, pullRequests chan []api.PullRequest, lastProcessed time.Time) error {
	return api.PaginateNewerThan(logger, lastProcessed, func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestPage(s.qc, repo, parameters, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}

		if err = pullRequestSender.SetTotal(pi.Total); err != nil {
			return pi, err
		}

		pullRequests <- res
		return pi, nil
	})
}
