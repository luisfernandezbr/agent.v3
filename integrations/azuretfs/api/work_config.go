package api

import (
	"fmt"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/integration-sdk/agent"
)

type workConfigRes struct {
	Name          string `json:"name"`
	ReferenceName string `json:"referenceName"`
	States        []struct {
		Category string `json:"category"`
		Name     string `json:"name"`
	} `json:"states"`
	FieldInstances []struct {
		ReferenceName string `json:"referenceName"`
	} `json:"fieldInstances"`
	Fields []struct {
		ReferenceName string `json:"referenceName"`
	} `json:"fields"`
}

// These seem to be the default statuses
const workConfigCompletedStatus = "Completed"
const workConfigInProgressStatus = "InProgress"
const workConfigProposedStatus = "Proposed"
const workConfigRemovedStatus = "Removed"
const workConfigResolvedStatus = "Resolved"

func stringEquals(str string, vals ...string) bool {
	for _, v := range vals {
		if str == v {
			return true
		}
	}
	return false
}
func (api *API) FetchWorkConfig() (*agent.WorkStatusResponseWorkConfig, error) {

	projects, err := api.FetchProjects(nil)
	if err != nil {
		return nil, err
	}
	var res []workConfigRes
	for _, project := range projects {
		var r []workConfigRes
		url := fmt.Sprintf(`%s/_apis/wit/workitemtypes`, project.RefID)
		if err = api.getRequest(url, stringmap{}, &r); err != nil {
			return nil, err
		}
		res = append(res, r...)
	}
	ws := &agent.WorkStatusResponseWorkConfig{}
	types := make(map[string]string)
	states := make(map[string]map[string]bool)
	for i, r := range res {
		// TODO: verify with other customer data
		// assuming the top level issue is the first one
		if i == 0 {
			ws.TopLevelIssue = agent.WorkStatusResponseWorkConfigTopLevelIssue{
				Name: r.Name,
				Type: r.ReferenceName,
			}
		}
		ws.Types = append(ws.Types, r.ReferenceName)
		types[r.ReferenceName] = r.Name

		for _, s := range r.States {
			if _, o := states[s.Category]; !o {
				states[s.Category] = make(map[string]bool)
			}
			states[s.Category][s.Name] = true
		}
	}

	for k, v := range states {
		if k == workConfigCompletedStatus || k == workConfigRemovedStatus || k == workConfigResolvedStatus {
			if k != workConfigRemovedStatus {
				for e := range v {
					ws.Resolutions.WorkDone = append(ws.Resolutions.WorkDone, e)
					ws.AllResolutions = append(ws.AllResolutions, e)
				}
			} else {
				for e := range v {
					ws.Resolutions.NoWorkDone = append(ws.Resolutions.NoWorkDone, e)
					ws.AllResolutions = append(ws.AllResolutions, e)
				}
			}
		} else {
			for e := range v {
				ws.AllStatuses = append(ws.AllStatuses, e)
			}
		}
	}

	for k := range states[workConfigProposedStatus] {
		ws.Statuses.OpenStatus = append(ws.Statuses.OpenStatus, k)
	}
	for k := range states[workConfigInProgressStatus] {
		ws.Statuses.InProgressStatus = append(ws.Statuses.InProgressStatus, k)
	}
	for k := range states[workConfigRemovedStatus] {
		ws.Statuses.ClosedStatus = append(ws.Statuses.ClosedStatus, k)
	}
	for k := range states[workConfigCompletedStatus] {
		ws.Statuses.ReleasedStatus = append(ws.Statuses.ReleasedStatus, k)
	}
	var enhancementRule agent.WorkStatusResponseWorkConfigTypeRules
	var bugRule agent.WorkStatusResponseWorkConfigTypeRules
	var featureRule agent.WorkStatusResponseWorkConfigTypeRules
	var otherRule agent.WorkStatusResponseWorkConfigTypeRules

	for refname := range types {
		predicate := agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
			Field:    agent.WorkStatusResponseWorkConfigTypeRulesPredicatesFieldType,
			Operator: agent.WorkStatusResponseWorkConfigTypeRulesPredicatesOperatorEquals,
			Value:    pstrings.Pointer(refname),
		}
		if stringEquals(refname,
			"Microsoft.VSTS.WorkItemTypes.Issue",
			"Microsoft.VSTS.WorkItemTypes.Bug",
			"Issue", "Bug") {
			bugRule.IssueType = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeBug
			bugRule.Predicates = append(bugRule.Predicates, predicate)
		} else if stringEquals(refname,
			"Microsoft.VSTS.WorkItemTypes.Task",
			"Task") {
			enhancementRule.IssueType = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeEnhancement
			enhancementRule.Predicates = append(enhancementRule.Predicates, predicate)
		} else if stringEquals(refname,
			"Microsoft.VSTS.WorkItemTypes.FeedbackRequest",
			"Microsoft.VSTS.WorkItemTypes.Feature",
			"Feature", "Feedback Request") {
			featureRule.IssueType = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeFeature
			featureRule.Predicates = append(featureRule.Predicates, predicate)
		} else {
			otherRule.IssueType = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeOther
			otherRule.Predicates = append(otherRule.Predicates, predicate)
		}
	}
	ws.TypeRules = []agent.WorkStatusResponseWorkConfigTypeRules{enhancementRule, bugRule, featureRule, otherRule}

	return ws, nil
}
