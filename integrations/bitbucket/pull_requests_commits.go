package main

import (
	"net/url"

	"github.com/pinpt/agent/integrations/bitbucket/api"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/integrations/pkg/objsender"

	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/hashicorp/go-hclog"
)

func (s *Integration) exportPullRequestCommits(logger hclog.Logger, repo commonrepo.Repo, pr sourcecode.PullRequest, prCommitsSender *objsender.Session) (res []*sourcecode.PullRequestCommit, rerr error) {

	params := url.Values{}
	params.Set("pagelen", "100")

	stopOnUpdatedAt := prCommitsSender.LastProcessedTime()

	rerr = api.Paginate(func(nextPage api.NextPage) (api.NextPage, error) {
		np, sub, err := api.PullRequestCommitsPage(s.qc, logger, repo, pr, params, stopOnUpdatedAt, nextPage)
		if err != nil {
			return np, err
		}
		for _, obj := range sub {
			res = append(res, obj)
		}
		return np, nil
	})

	return
}
