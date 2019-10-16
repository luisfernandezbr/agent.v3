package main

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	err := s.initWithConfig(config)
	if err != nil {
		return res, err
	}
	switch objectType {
	case rpcdef.OnboardExportTypeRepos:
		return s.onboardExportRepos(ctx)
	default:
		res.Error = rpcdef.ErrOnboardExportNotSupported
		return
	}
}

func (s *Integration) onboardExportRepos(ctx context.Context) (res rpcdef.OnboardExportResult, _ error) {
	groupNames, err := api.Groups(s.qc)
	if err != nil {
		return res, err
	}

	var records []map[string]interface{}

	for _, groupName := range groupNames {
		api.PaginateGraphQL(s.logger, func(log hclog.Logger, pageSize string, after string) (string, error) {
			afterCursor, repos, err := api.ReposOnboardPageGraphQL(s.qc, groupName, pageSize, after)
			if err != nil {
				return "", err
			}
			for _, repo := range repos {
				records = append(records, repo.ToMap())
			}
			return afterCursor, nil
		})
	}

	res.Data = records

	return res, nil
}
