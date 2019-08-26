package main

import (
	"time"

	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent.next/integrations/github/api"
)

func (s *Integration) exportPullRequests(
	repos []api.Repo,
	pullRequests chan []api.PullRequest) error {

	sender, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestModelName.String())
	if err != nil {
		return err
	}
	defer sender.Done()

	for _, repo := range repos {
		//if i > 1 {
		//	break
		//}
		err := s.exportPullRequestsRepo(sender, repo, pullRequests)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Integration) exportPullRequestsRepo(sender *objsender.IncrementalDateBased, repo api.Repo, pullRequests chan []api.PullRequest) error {
	return api.PaginateNewerThan(sender.LastProcessed, func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestsPage(s.qc, repo.ID, query, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}

		pullRequests <- res

		var batch []objsender.Model
		for _, obj := range res {
			batch = append(batch, obj)
		}
		return pi, sender.Send(batch)
	})
}
