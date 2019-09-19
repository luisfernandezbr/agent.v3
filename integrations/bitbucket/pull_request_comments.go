package main

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/bitbucket/api"
	"github.com/pinpt/agent.next/pkg/commonrepo"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsComments(logger hclog.Logger, sender *objsender.NotIncremental, repo commonrepo.Repo, pullRequests chan []sourcecode.PullRequest) error {
	for prs := range pullRequests {
		for _, pr := range prs {
			err := s.exportPullRequestComments(logger, sender, repo, pr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestComments(logger hclog.Logger, sender *objsender.NotIncremental, repo commonrepo.Repo, pr sourcecode.PullRequest) error {
	return api.Paginate(logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		pi, res, err := api.PullRequestCommentsPage(s.qc, repo, pr, paginationParams)
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
