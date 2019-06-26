package main

import (
	"context"
	"time"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) exportRepos(ctx context.Context) error {
	et, err := s.newExportType("sourcecode.repo")
	if err != nil {
		return err
	}
	defer et.Done()

	return et.Paginate(func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, repos, err := api.ReposPage(s.qc, query, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		batch := []rpcdef.ExportObj{}
		for _, repo := range repos {
			batch = append(batch, rpcdef.ExportObj{Data: repo.ToMap()})
		}
		return pi, et.Send(batch)
	})
}
