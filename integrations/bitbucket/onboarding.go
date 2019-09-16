package main

import (
	"context"

	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	// err := s.initWithConfig(config)
	// if err != nil {
	// 	return res, err
	// }
	// switch objectType {
	// case rpcdef.OnboardExportTypeUsers:
	// 	return s.onboardExportUsers(ctx)
	// case rpcdef.OnboardExportTypeRepos:
	// 	return s.onboardExportRepos(ctx)
	// default:
	// 	res.Error = rpcdef.ErrOnboardExportNotSupported
	// 	return
	// }
	return
}
