package main

import (
	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestComments(pullRequests chan []api.PullRequest) error {
	sender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestCommentModelName.String())

	for prs := range pullRequests {
		for _, pr := range prs {
			if !pr.HasComments {
				// perf optimization
				continue
			}
			err := s.exportPullRequestCommentsPR(sender, pr.RefID)
			if err != nil {
				return err
			}
		}
	}
	return sender.Done()
}

func (s *Integration) exportPullRequestCommentsPR(sender *objsender.NotIncremental, prID string) error {
	return api.PaginateRegular(func(query string) (api.PageInfo, error) {
		pi, res, err := api.PullRequestCommentsPage(s.qc, prID, query)
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
