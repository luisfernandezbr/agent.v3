package main

import (
	"context"
	"strconv"

	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	switch objectType {
	case rpcdef.OnboardExportTypeProjects:
		return s.onboardProjects(ctx, objectType, config)
	default:
		res.Error = rpcdef.ErrOnboardExportNotSupported
		return
	}
}

func (s *Integration) onboardProjects(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	var rows []map[string]interface{}
	for j := 0; j < 10; j++ {
		row := agent.ProjectResponseProjects{}
		row.Name = "Project " + strconv.Itoa(j)
		rows = append(rows, row.ToMap())
	}
	res.Data = rows
	return
}
