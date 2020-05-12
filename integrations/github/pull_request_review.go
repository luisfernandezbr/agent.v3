package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsReviews(logger hclog.Logger, prSender *objsender.Session, repo api.Repo, pullRequests chan []api.PullRequest) error {
	for prs := range pullRequests {
		for _, pr := range prs {
			if !pr.HasReviews {
				// perf optimization
				continue
			}
			reviewsSender, err := prSender.Session(sourcecode.PullRequestReviewModelName.String(), pr.RefID, pr.RefID)
			if err != nil {
				return err
			}
			err = s.exportPullRequestReviews(logger, reviewsSender, repo, pr.RefID)
			if err != nil {
				return err
			}
			err = reviewsSender.Done()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestReviews(logger hclog.Logger, reviewsSender objsender.SessionCommon, repo api.Repo, prID string) error {
	return api.PaginateRegularWithPageSize(pageSizeHeavyQueries, func(query string) (api.PageInfo, error) {
		pi, res, totalCount, err := api.PullRequestReviewTimelineItemsPage(s.qc, repo, prID, query)
		if err != nil {
			return pi, err
		}
		err = reviewsSender.SetTotal(totalCount)
		if err != nil {
			return pi, err
		}
		for _, obj := range res {
			err := reviewsSender.Send(obj)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})
}
