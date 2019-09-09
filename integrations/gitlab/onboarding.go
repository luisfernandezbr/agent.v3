package main

import (
	"context"
	"net/url"

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
	case rpcdef.OnboardExportTypeUsers:
		return s.onboardExportUsers(ctx)
	case rpcdef.OnboardExportTypeRepos:
		return s.onboardExportRepos(ctx)
	default:
		res.Error = rpcdef.ErrOnboardExportNotSupported
		return
	}
	return rpcdef.OnboardExportResult{}, nil
}

func (s *Integration) onboardExportRepos(ctx context.Context) (res rpcdef.OnboardExportResult, _ error) {
	groupNames, err := api.Groups(s.qc)
	if err != nil {
		return res, err
	}

	for _, groupName := range groupNames {
		api.PaginateGraphQL(s.logger, func(log hclog.Logger, pageSize string, after string) (string, error) {
			afterCursor, repos, err := api.ReposOnboardPageGraphQL(s.qc, groupName, pageSize, after)
			if err != nil {
				return "", err
			}
			for _, repo := range repos {
				res.Records = append(res.Records, repo.ToMap())
			}
			return afterCursor, nil
		})
	}

	return res, nil
}

func (s *Integration) onboardExportUsers(ctx context.Context) (res rpcdef.OnboardExportResult, _ error) {

	api.PaginateStartAt(s.logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		page, users, err := api.UsersOnboardPage(s.qc, paginationParams)
		if err != nil {
			return page, err
		}
		for _, user := range users {
			res.Records = append(res.Records, user.ToMap())
		}
		return page, nil
	})

	return res, nil
}
