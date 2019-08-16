package main

import (
	"context"

	"github.com/pinpt/agent.next/integrations/jira-hosted/api"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	switch objectType {
	case rpcdef.OnboardExportTypeUsers:
		return s.onboardExportUsers(ctx, config)
	case rpcdef.OnboardExportTypeProjects:
		return s.onboardExportProjects(ctx, config)
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
	users, err := api.UsersOnboard(s.qc)
	if err != nil {
		return res, err
	}
	for _, u := range users {
		res.Records = append(res.Records, u.ToMap())
	}
	return res, nil
}

func (s *Integration) onboardExportProjects(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	err := s.initWithConfig(config)
	if err != nil {
		return res, err
	}
	projects, err := api.ProjectsOnboard(s.qc)
	if err != nil {
		return res, err
	}
	for _, obj := range projects {
		res.Records = append(res.Records, obj.ToMap())
	}
	return res, nil
}
