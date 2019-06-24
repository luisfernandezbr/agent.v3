package main

import (
	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) exportPullRequestReviews(pullRequests chan []api.PullRequest) error {
	et, err := s.newExportType("sourcecode.pull_request_review")
	if err != nil {
		return err
	}
	defer et.Done()

	for prs := range pullRequests {
		for _, pr := range prs {
			if !pr.HasReviews {
				// perf optimization
				continue
			}
			err := s.exportPullRequestReviewsPR(et, pr.RefID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportPullRequestReviewsPR(et *exportType, prID string) error {
	return api.PaginateRegular(func(query string) (api.PageInfo, error) {
		pi, res, err := api.PullRequestReviewsPage(s.qc, prID, query)
		if err != nil {
			return pi, err
		}
		batch := []rpcdef.ExportObj{}
		//var ids []string
		for _, obj := range res {
			//ids = append(ids, obj.ID)
			batch = append(batch, rpcdef.ExportObj{Data: obj.ToMap()})
		}
		//resIDs <- ids
		return pi, et.Send(batch)
	})
}
