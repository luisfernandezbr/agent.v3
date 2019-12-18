package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsComments(logger hclog.Logger, prSender *objsender.Session, pullRequests chan []api.PullRequest) error {
	for prs := range pullRequests {
		for _, pr := range prs {
			if !pr.HasComments {
				// perf optimization
				continue
			}
			err := s.exportPullRequestComments(logger, prSender, pr.RefID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestComments(logger hclog.Logger, prSender *objsender.Session, prID string) error {
	commentsSender, err := prSender.Session(sourcecode.PullRequestCommentModelName.String(), prID, prID)
	if err != nil {
		return err
	}

	err = api.PaginateRegularWithPageSize(pageSizeHeavyQueries, func(query string) (api.PageInfo, error) {
		pi, res, totalCount, err := api.PullRequestCommentsPage(s.qc, prID, query)
		if err != nil {
			return pi, err
		}

		err = commentsSender.SetTotal(totalCount)
		if err != nil {
			return pi, err
		}

		for _, obj := range res {
			err := commentsSender.Send(obj)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})

	if err != nil {
		return err
	}

	return commentsSender.Done()
}
