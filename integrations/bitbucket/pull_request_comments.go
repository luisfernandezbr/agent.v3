package main

import (
	"fmt"
	"net/url"

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

	params := url.Values{}
	params.Set("pagelen", "100")

	stopOnUpdatedAt := commentsSender.LastProcessedTime()
	if !stopOnUpdatedAt.IsZero() {
		params.Set("q", fmt.Sprintf(" updated_on > %s", stopOnUpdatedAt.UTC().Format("2006-01-02T15:04:05.000000-07:00")))
	}

	return api.Paginate(logger, func(log hclog.Logger, nextPage api.NextPage) (np api.NextPage, _ error) {
		pi, res, err := api.PullRequestCommentsPage(s.qc, repo, pr, params, nextPage)
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
