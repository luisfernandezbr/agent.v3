package main

import (
	"context"
	"time"

	"github.com/pinpt/agent.next/pkg/objsender"

	"github.com/pinpt/agent.next/integrations/github/api"
)

func (s *Integration) exportRepos(ctx context.Context) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, "sourcecode.repo")
	if err != nil {
		return err
	}
	defer sender.Done()

	return api.PaginateNewerThan(sender.LastProcessed, func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, repos, err := api.ReposPage(s.qc, query, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		var batch []objsender.Model
		for _, repo := range repos {
			batch = append(batch, repo)
		}
		return pi, sender.Send(batch)
	})
}
