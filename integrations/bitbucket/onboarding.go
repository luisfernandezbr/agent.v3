package main

import (
	"context"
	"net/url"

	"github.com/pinpt/agent/integrations/bitbucket/api"
	"github.com/pinpt/agent/rpcdef"
)

// OnboardExport onboard export
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

func (s *Integration) onboardExportRepos(ctx context.Context) (res rpcdef.OnboardExportResult, rerr error) {
	var records []map[string]interface{}

	params := url.Values{}
	params.Set("pagelen", "100")

	rerr = api.Paginate(func(nextPage api.NextPage) (np api.NextPage, _ error) {
		pageInfo, repos, err := api.ResposUserHasAccessToPage(s.qc, params, nextPage)
		if err != nil {
			return np, err
		}
		for _, repo := range repos {
			records = append(records, repo.ToMap())
		}
		return pageInfo, nil
	})
	if rerr != nil {
		return
	}

	res.Data = records

	return
}
