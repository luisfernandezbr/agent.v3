package main

import (
	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestReviews(pullRequests chan []api.PullRequest) error {
	sender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestReviewModelName.String())

	for prs := range pullRequests {
		for _, pr := range prs {
			if !pr.HasReviews {
				// perf optimization
				continue
			}
			err := s.exportPullRequestReviewsPR(sender, pr.RefID)
			if err != nil {
				return err
			}
		}
	}
	return sender.Done()
}

func (s *Integration) exportPullRequestReviewsPR(sender *objsender.NotIncremental, prID string) error {
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
