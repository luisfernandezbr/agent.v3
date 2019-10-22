package main

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
	"github.com/pinpt/agent.next/integrations/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsReviews(logger hclog.Logger, prSender *objsender.Session, repo commonrepo.Repo, pullRequests chan []api.PullRequest) error {
	for prs := range pullRequests {
		for _, pr := range prs {
			err := s.exportPullRequestReviews(logger, prSender, repo, pr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestReviews(logger hclog.Logger, prSender *objsender.Session, repo commonrepo.Repo, pr api.PullRequest) error {

	reviewsSender, err := prSender.Session(sourcecode.PullRequestReviewModelName.String(), pr.RefID, pr.RefID)
	if err != nil {
		return err
	}

	err = api.PaginateStartAt(logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		pi, res, err := api.PullRequestReviewsPage(s.qc, repo, pr, paginationParams)
		if err != nil {
			return pi, err
		}

		if err = reviewsSender.SetTotal(pi.Total); err != nil {
			return pi, err
		}

		for _, obj := range res {
			if err := reviewsSender.Send(obj); err != nil {
				return pi, err
			}
		}

		return pi, nil

	})
	if err != nil {
		return err
	}

	return reviewsSender.Done()
}
