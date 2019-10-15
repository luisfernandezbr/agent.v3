package main

import (
	"net/url"

	"github.com/pinpt/agent.next/integrations/pkg/objsender2"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/bitbucket/api"
	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsComments(logger hclog.Logger, prSender *objsender2.Session, repo commonrepo.Repo, pullRequests chan []sourcecode.PullRequest) error {
	for prs := range pullRequests {
		for _, pr := range prs {
			err := s.exportPullRequestComments(logger, prSender, repo, pr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestComments(logger hclog.Logger, prSender *objsender2.Session, repo commonrepo.Repo, pr sourcecode.PullRequest) error {

	commentsSender, err := prSender.Session(sourcecode.PullRequestCommentModelName.String(), pr.RefID, pr.RefID)
	if err != nil {
		return err
	}

	err = api.Paginate(logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		pi, res, err := api.PullRequestCommentsPage(s.qc, repo, pr, paginationParams)
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

	if err != nil {
		return err
	}

	return commentsSender.Done()
}
