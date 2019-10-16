package main

import (
	"context"
	"net/url"

	"github.com/pinpt/agent.next/integrations/jira-cloud/api"
	"github.com/pinpt/agent.next/integrations/pkg/jiracommon"
	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	switch objectType {
	case rpcdef.OnboardExportTypeProjects:
		return s.onboardExportProjects(ctx, config)
	case rpcdef.OnboardExportTypeWorkConfig:
		return s.onboardWorkConfig(ctx, config)
	default:
		res.Error = rpcdef.ErrOnboardExportNotSupported
		return
	}
}

func (s *Integration) onboardExportProjects(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, rerr error) {
	err := s.initWithConfig(config)
	if err != nil {
		rerr = err
		return
	}
	var records []map[string]interface{}
	err = jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, rerr error) {
		pi, projects, err := api.ProjectsOnboardPage(s.qc, paginationParams)
		if err != nil {
			rerr = err
			return
		}
		for _, obj := range projects {
			records = append(records, obj.ToMap())
		}
		return pi.HasMore, pi.MaxResults, nil
	})
	if err != nil {
		rerr = err
		return
	}
	res.Data = records
	return res, nil
}

func (s *Integration) onboardWorkConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {

	err := s.initWithConfig(config)
	if err != nil {
		return res, err
	}

	return jiracommon.GetWorkConfig(s.qc.Common(), "cloud")
}
