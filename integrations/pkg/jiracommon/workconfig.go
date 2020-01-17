package jiracommon

import (
	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/rpcdef"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func GetWorkConfig(qc jiracommonapi.QueryContext, isCloud bool, usesOauth bool) (res rpcdef.OnboardExportResult, _ error) {

	var ws agent.WorkStatusResponseWorkConfig

	priorities, err := jiracommonapi.Priorities(qc)
	if err != nil {
		res.Error = err
		return
	}
	ws.Priorities = priorities

	if isCloud && !usesOauth {
		// This doesn't work with hosted
		// This doesn't work with cloud with oauth, server response: "OAuth 2.0 is not enabled for this method."
		labels, err := jiracommonapi.Labels(qc)
		if err != nil {
			res.Error = err
			return
		}
		ws.Labels = labels
	}

	issueTypes, err := jiracommonapi.IssueTypes(qc)
	if err != nil {
		res.Error = err
		return
	}
	ws.Types = issueTypes

	statusDetail, allStatus, err := jiracommonapi.StatusWithDetail(qc)
	if err != nil {
		res.Error = err
		return
	}
	ws.AllStatuses = allStatus

	resolutions, err := jiracommonapi.Resolution(qc)
	if err != nil {
		res.Error = err
		return
	}

	ws.AllResolutions = resolutions

	appendStaticInfo(&ws, statusDetail)

	res.Data = ws.ToMap()
	return
}

func getExistedStatusesOnly(allstatus, optns []string) []string {

	var res []string
	mAll := make(map[string]bool)

	for _, v := range allstatus {
		mAll[v] = true
	}

	for _, v := range optns {
		if _, ok := mAll[v]; ok {
			res = append(res, v)
		}
	}

	return res
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
	ws.Resolutions.WorkDone = []string{"Completed"}
	ws.Resolutions.NoWorkDone = []string{"Won't Do", "Invalid"}
	ws.TypeRules = []agent.WorkStatusResponseWorkConfigTypeRules{
		agent.WorkStatusResponseWorkConfigTypeRules{
			IssueType: agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeFeature,
			Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
				agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
					Field:    agent.WorkStatusResponseWorkConfigTypeRulesPredicatesFieldType,
					Operator: agent.WorkStatusResponseWorkConfigTypeRulesPredicatesOperatorEquals,
					Value:    pstrings.Pointer("Feature"),
				},
			},
		},
		agent.WorkStatusResponseWorkConfigTypeRules{
			IssueType: agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeEnhancement,
			Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
				agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
					Field:    agent.WorkStatusResponseWorkConfigTypeRulesPredicatesFieldType,
					Operator: agent.WorkStatusResponseWorkConfigTypeRulesPredicatesOperatorEquals,
					Value:    pstrings.Pointer("Enhancement"),
				},
			},
		},
		agent.WorkStatusResponseWorkConfigTypeRules{
			IssueType: agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeBug,
			Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
				agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
					Field:    agent.WorkStatusResponseWorkConfigTypeRulesPredicatesFieldType,
					Operator: agent.WorkStatusResponseWorkConfigTypeRulesPredicatesOperatorEquals,
					Value:    pstrings.Pointer("Bug"),
				},
			},
		},
	}
}
