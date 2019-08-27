package main

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent.next/integrations/github/api"
)

func (s *Integration) exportPullRequests(
	logger hclog.Logger,
	repos []api.Repo,
	pullRequests chan []api.PullRequest) error {

	sender, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestModelName.String())
	if err != nil {
		return err
	}
	for _, repo := range repos {
		err := s.exportPullRequestsRepo(logger, sender, repo, pullRequests)
		if err != nil {
			return err
		}
	}
	return sender.Done()
}

func (s *Integration) exportPullRequestsRepo(logger hclog.Logger, sender *objsender.IncrementalDateBased, repo api.Repo, pullRequests chan []api.PullRequest) error {
	return api.PaginateNewerThan(sender.LastProcessed, func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestsPage(s.qc, repo.ID, query, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}

		pullRequests <- res

		for _, obj := range res {
			err := sender.Send(obj)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})
}
