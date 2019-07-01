package main

import (
	"time"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) exportPullRequests(
	repos []api.Repo,
	pullRequests chan []api.PullRequest) error {
	et, err := s.newExportType("sourcecode.pull_request")
	if err != nil {
		return err
	}
	defer et.Done()

	for _, repo := range repos {
		//if i > 1 {
		//	break
		//}
		err := s.exportPullRequestsRepo(et, repo, pullRequests)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Integration) exportPullRequestsRepo(et *exportType, repo api.Repo, pullRequests chan []api.PullRequest) error {

	return et.Paginate(func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestsPage(s.qc, repo.ID, query, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}

		pullRequests <- res

		batch := []rpcdef.ExportObj{}
		for _, obj := range res {
			batch = append(batch, rpcdef.ExportObj{Data: obj.ToMap()})
		}
		return pi, et.Send(batch)
	})
}
