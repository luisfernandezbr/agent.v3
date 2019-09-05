package main

import (
	"context"

	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	// switch objectType {
	// case rpcdef.OnboardExportTypeUsers:
	// 	return s.onboardExportUsers(ctx, config)
	// case rpcdef.OnboardExportTypeRepos:
	// 	return s.onboardExportRepos(ctx, config)
	// default:
	// 	res.Error = rpcdef.ErrOnboardExportNotSupported
	// 	return
	// }
	return rpcdef.OnboardExportResult{}, nil
}
