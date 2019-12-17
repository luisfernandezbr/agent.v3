package main

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/agent/integrations/pkg/objsender"
)

func (s *Integration) exportRepos(ctx context.Context, logger hclog.Logger, sender *objsender.Session, org api.Org, onlyInclude []api.Repo) error {

	// map[nameWithOwner]shouldInclude
	shouldInclude := map[string]bool{}
	for _, repo := range onlyInclude {
		shouldInclude[repo.NameWithOwner] = true
	}

	err := api.PaginateNewerThan(sender.LastProcessedTime(), func(query string, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, repos, _, err := api.ReposPage(s.qc.WithLogger(logger), org, query, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		for _, repo := range repos {
			// sourcecode.Repo.Name == api.Repo.NameWithOwner
			if !shouldInclude[repo.Name] {
				continue
			}
			err := sender.Send(repo)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})

	if err != nil {
		return err
	}

	return nil
}
