package main

import (
	"context"

	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	switch objectType {
	case rpcdef.OnboardExportTypeUsers:
		return s.onboardExportUsers(ctx, config)
	case rpcdef.OnboardExportTypeRepos:
		return s.onboardExportRepos(ctx, config)
	default:
		res.Error = rpcdef.ErrOnboardExportNotSupported
		return
	}
}

func (s *Integration) onboardExportUsers(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	err := s.initWithConfig(config)
	if err != nil {
		return res, err
	}

	orgs, err := s.getOrgs()
	if err != nil {
		return res, err
	}

	resChan := make(chan []*sourcecode.User)

	go func() {
		for _, org := range orgs {
			defer close(resChan)
			err := api.UsersAll(s.qc, org, resChan)
			if err != nil {
				panic(err)
			}
		}
	}()

	for users := range resChan {
		for _, u1 := range users {
			u2 := agent.UserResponseUsers{}
			u2.RefType = u1.RefType
			u2.RefID = u1.RefID
			u2.CustomerID = u1.CustomerID
			u2.AvatarURL = u1.AvatarURL
			u2.Name = u1.Name
			if u1.Email != nil {
				u2.Emails = []string{*u1.Email}
			}
			res.Records = append(res.Records, u2.ToMap())
		}
	}

	return res, nil
}

func (s *Integration) onboardExportRepos(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	err := s.initWithConfig(config)
	if err != nil {
		return res, err
	}

	orgs, err := s.getOrgs()
	if err != nil {
		return res, err
	}

	for _, org := range orgs {
		repos, err := api.ReposForOnboardAll(s.qc, org)
		if err != nil {
			return res, err
		}
		for _, r := range repos {
			res.Records = append(res.Records, r.ToMap())
		}
	}

	return res, nil
}
