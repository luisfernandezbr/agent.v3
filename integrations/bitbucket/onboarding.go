package main

import (
	"context"
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/bitbucket/api"
	"github.com/pinpt/agent/rpcdef"
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
	teamNames, err := api.Teams(s.qc)
	if err != nil {
		return res, err
	}
	var records []map[string]interface{}

	for _, teamName := range teamNames {
		api.Paginate(s.logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
			pageInfo, repos, err := api.ReposOnboardPage(s.qc, teamName, paginationParams)
			if err != nil {
				return page, err
			}
			for _, repo := range repos {
				records = append(records, repo.ToMap())
			}
			return pageInfo, nil
		})
	}

	res.Data = records

	return res, nil
}
