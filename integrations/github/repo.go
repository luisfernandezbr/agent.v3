package main

import (
	"context"
	"time"

	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent.next/integrations/github/api"
)

func (s *Integration) exportRepos(ctx context.Context, org api.Org, excludedByNameWithOwner []string) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.RepoModelName.String())
	if err != nil {
		return err
	}
	defer sender.Done()

	excludedMap := map[string]bool{}
	for _, no := range excludedByNameWithOwner {
		excludedMap[no] = true
	}

	return api.PaginateNewerThan(sender.LastProcessed, func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, repos, err := api.ReposPage(s.qc, org, query, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		var batch []objsender.Model
		for _, repo := range repos {
			if excludedMap[repo.Name] {
				continue
			}
			batch = append(batch, repo)
		}
		return pi, sender.Send(batch)
	})
}
