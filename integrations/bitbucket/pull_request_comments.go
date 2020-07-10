package main

import (
	"net/url"
	"time"

	"github.com/pinpt/agent/integrations/pkg/objsender"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/bitbucket/api"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsComments(logger hclog.Logger, commentsSender *objsender.Session, repo commonrepo.Repo, pullRequests chan []sourcecode.PullRequest) error {
	for prs := range pullRequests {
		for _, pr := range prs {
			err := s.exportPullRequestComments(logger, commentsSender, repo, pr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestComments(logger hclog.Logger, commentsSender *objsender.Session, repo commonrepo.Repo, pr sourcecode.PullRequest) error {

	return api.PaginateNewerThan(logger, commentsSender.LastProcessedTime(), func(log hclog.Logger, paginationParams url.Values, stopOnUpdatedAt time.Time) (page api.PageInfo, _ error) {
		pi, res, err := api.PullRequestCommentsPage(s.qc, repo, pr, paginationParams, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}

		err = commentsSender.SetTotal(pi.Total)
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

}
