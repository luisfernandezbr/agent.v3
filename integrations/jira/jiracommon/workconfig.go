package jiracommon

import (
	"github.com/pinpt/agent/integrations/jira/jiracommonapi"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/agent"
)

// GetWorkConfig will return the default jira work config
func GetWorkConfig(qc jiracommonapi.QueryContext) (res rpcdef.OnboardExportResult, _ error) {

	var ws agent.WorkStatusResponseWorkConfig

	statusDetail, _, err := jiracommonapi.StatusWithDetail(qc)
	if err != nil {
		res.Error = err
		return
	}

	appendStaticInfo(&ws, statusDetail)

	res.Data = ws.ToMap()
	return
}

func appendStaticInfo(ws *agent.WorkStatusResponseWorkConfig, statuses []jiracommonapi.StatusDetail) {
	ws.Statuses = agent.WorkStatusResponseWorkConfigStatuses{}
	found := make(map[string]bool)
	for _, status := range statuses {
		if !found[status.Name] {
			found[status.Name] = true
			switch status.StatusCategory.Key {
			case "new":
				ws.Statuses.OpenStatus = append(ws.Statuses.OpenStatus, status.Name)
			case "done":
				ws.Statuses.ClosedStatus = append(ws.Statuses.ClosedStatus, status.Name)
			case "indeterminate":
				ws.Statuses.InProgressStatus = append(ws.Statuses.InProgressStatus, status.Name)
			}
		}
	}
	ws.TopLevelIssue = agent.WorkStatusResponseWorkConfigTopLevelIssue{
		Name: "Epic",
		Type: "Issue",
	}
}
