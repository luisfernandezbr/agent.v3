package main

import (
	"context"

	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/agent/rpcdef"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	switch objectType {
	case rpcdef.OnboardExportTypeRepos:
		return s.onboardExportRepos(ctx, config)
	default:
		res.Error = rpcdef.ErrOnboardExportNotSupported
		return
	}
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

	var records []map[string]interface{}
	for _, org := range orgs {
		repos, err := api.ReposForOnboardAll(s.qc, org)
		if err != nil {
			return res, err
		}
		for _, r := range repos {
			records = append(records, r.ToMap())
		}
	}

	if len(orgs) == 0 {
		// personal repos
		repos, err := api.ReposForOnboardAll(s.qc, api.Org{})
		if err != nil {
			return res, err
		}
		for _, r := range repos {
			records = append(records, r.ToMap())
		}
	}

	res.Data = records

	return res, nil
}
