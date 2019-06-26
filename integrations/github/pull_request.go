package main

import (
	"time"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) exportPullRequests(
	repoIDs []string,
	pullRequests chan []api.PullRequest) error {
	et, err := s.newExportType("sourcecode.pull_request")
	if err != nil {
		return err
	}
	defer et.Done()

	for _, repoID := range repoIDs {
		err := s.exportPullRequestsRepo(et, repoID, pullRequests)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Integration) exportPullRequestsRepo(et *exportType, repoID string, pullRequests chan []api.PullRequest) error {

	return et.Paginate(func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestsPage(s.qc, repoID, query, stopOnUpdatedAt)
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
