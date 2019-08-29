package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/pkg/objsender"
)

func (s *Integration) exportPullRequestsReviews(logger hclog.Logger, sender *objsender.NotIncremental, pullRequests chan []api.PullRequest) error {
	for prs := range pullRequests {
		for _, pr := range prs {
			if !pr.HasReviews {
				// perf optimization
				continue
			}
			err := s.exportPullRequestReviews(logger, sender, pr.RefID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestReviews(logger hclog.Logger, sender *objsender.NotIncremental, prID string) error {
	return api.PaginateRegular(func(query string) (api.PageInfo, error) {
		pi, res, err := api.PullRequestReviewsPage(s.qc, prID, query)
		if err != nil {
			return pi, err
		}
		for _, obj := range res {
			err := sender.Send(obj)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})
}
