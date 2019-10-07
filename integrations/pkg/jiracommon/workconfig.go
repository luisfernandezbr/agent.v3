package jiracommon

import (
	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent.next/rpcdef"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func GetWorkConfig(qc jiracommonapi.QueryContext) (res rpcdef.OnboardExportResult, _ error) {

	ws := GetStaticWorkConfig()

	priorities, err := jiracommonapi.Priorities(qc)
	if err != nil {
		res.Error = err
		return
	}
	ws.Priorities = priorities

	labels, err := jiracommonapi.Labels(qc)
	if err != nil {
		res.Error = err
		return
	}
	ws.Labels = labels

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

	res.Records = append(res.Records, ws.ToMap())
	return
}

func GetStaticWorkConfig() (ws agent.WorkStatusResponseWorkConfig) {
	ws.Statuses = agent.WorkStatusResponseWorkConfigStatuses{
		ClosedStatus:     []string{"Work Complete", "Completed", "Closed", "Done", "Fixed"},
		CompletedStatus:  []string{"Work Complete", "Completed", "Done", "Fixed", "Validated", "Evidence Validated"},
		InProgressStatus: []string{"In Progress", "On Hold"},
		OpenStatus:       []string{"Open", "Work Required", "To Do", "Backlog"},
		InReviewStatus:   []string{"Awaiting Release", "Awaiting Validation", "In Review"},
		ReleasedStatus:   []string{"Released"},
		ReopenedStatus:   []string{"Reopened", "Rework"},
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
	return
}
