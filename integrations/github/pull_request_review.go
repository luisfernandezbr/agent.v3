package main

import (
	"strings"

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

		if !s.isAssigneeAvailableSet() {
			s.logger.Info("check assignee availability")
			_, _, _, err := api.PullRequestReviewTimelineItemsPage(s.qc, repo, prID, query, true)
			if err != nil {
				if strings.Contains(err.Error(), "Field 'assignee' doesn't exist on type 'AssignedEvent'") {
					s.logger.Info("setting assignee availability", "status", false)
					s.setAssigneeAvailability(false)
				}
			} else {
				s.logger.Info("setting assignee availability", "status", false)
				s.setAssigneeAvailability(true)
			}
		}

		pi, res, totalCount, err := api.PullRequestReviewTimelineItemsPage(s.qc, repo, prID, query, s.isAssigneeAvailable())
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
