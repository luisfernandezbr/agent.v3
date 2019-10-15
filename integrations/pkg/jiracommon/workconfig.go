package jiracommon

import (
	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent.next/rpcdef"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func GetWorkConfig(qc jiracommonapi.QueryContext, typeServer string) (res rpcdef.OnboardExportResult, _ error) {

	var ws agent.WorkStatusResponseWorkConfig

	priorities, err := jiracommonapi.Priorities(qc)
	if err != nil {
		res.Error = err
		return
	}
	ws.Priorities = priorities

	if typeServer == "cloud" {
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

	allStatus, err := jiracommonapi.Status(qc)
	if err != nil {
		res.Error = err
		return
	}
	ws.AllStatuses = allStatus

	resolutions, err := jiracommonapi.Resolution(qc)
	ws.AllResolutions = resolutions

	appendStaticInfo(&ws)

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

func appendStaticInfo(ws *agent.WorkStatusResponseWorkConfig) {
	ws.Statuses = agent.WorkStatusResponseWorkConfigStatuses{
		ClosedStatus:     getExistedStatusesOnly(ws.AllStatuses, []string{"Work Complete", "Completed", "Closed", "Done", "Fixed"}),
		InProgressStatus: getExistedStatusesOnly(ws.AllStatuses, []string{"In Progress", "On Hold"}),
		OpenStatus:       getExistedStatusesOnly(ws.AllStatuses, []string{"Open", "Work Required", "To Do", "Backlog"}),
		InReviewStatus:   getExistedStatusesOnly(ws.AllStatuses, []string{"Awaiting Release", "Awaiting Validation", "In Review"}),
		ReleasedStatus:   getExistedStatusesOnly(ws.AllStatuses, []string{"Released"}),
		ReopenedStatus:   getExistedStatusesOnly(ws.AllStatuses, []string{"Reopened", "Rework"}),
	}
	ws.TopLevelIssue = agent.WorkStatusResponseWorkConfigTopLevelIssue{
		Name: "Epic",
		Type: "Issue",
	}
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
