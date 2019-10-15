package main

import (
	"context"
	"strconv"

	"github.com/pinpt/integration-sdk/customer"

	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	switch objectType {
	case rpcdef.OnboardExportTypeUsers:
		return s.onboardUsers(ctx, objectType, config)
	case rpcdef.OnboardExportTypeProjects:
		return s.onboardProjects(ctx, objectType, config)
	default:
		res.Error = rpcdef.ErrOnboardExportNotSupported
		return
	}
}

func (s *Integration) onboardUsers(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	var users []map[string]interface{}
	for j := 0; j < 10; j++ {
		row := customer.User{}
		row.Name = "User " + strconv.Itoa(j)
		users = append(users, row.ToMap())
	}
	var teams []map[string]interface{}
	for j := 0; j < 3; j++ {
		row := customer.Team{}
		row.Name = "Team " + strconv.Itoa(j)
		teams = append(teams, row.ToMap())
	}
	res.Data = map[string]interface{}{
		"users": users,
		"teams": teams,
	}
	return
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
