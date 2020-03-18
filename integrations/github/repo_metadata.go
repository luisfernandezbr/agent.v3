package main

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/agent/integrations/pkg/objsender"
)

func (s *Integration) exportRepoMetadata(sender *objsender.Session, orgs []api.Org, onlyInclude []Repo) error {
	for _, org := range orgs {
		logger := s.logger.With("org", org.Login)
		err := s.exportRepoMetadataOrg(logger, sender, org, onlyInclude)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Integration) exportRepoMetadataOrg(logger hclog.Logger, sender *objsender.Session, org api.Org, onlyInclude []Repo) error {

	// map[nameWithOwner]shouldInclude
	shouldInclude := map[string]bool{}
	for _, repo := range onlyInclude {
		shouldInclude[repo.NameWithOwner] = true
	}

	err := api.PaginateRegular(func(query string) (api.PageInfo, error) {
		pi, repos, _, err := api.ReposPage(s.qc.WithLogger(logger), org, query, time.Time{})
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

	err = api.PaginateRegular(func(query string) (api.PageInfo, error) {
		pi, repos, _, err := api.ReposPage(s.qc.WithLogger(logger), api.Org{}, query, time.Time{})
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
