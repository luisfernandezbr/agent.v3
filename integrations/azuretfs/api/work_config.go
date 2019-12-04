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

type workConfigClean struct {
	RefName string
	States  map[string]string // map[name]category
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

func appendString(arr []string, item string) []string {
	if !exists(item, arr) {
		arr = append(arr, item)
	}
	return arr
}

func (api *API) FetchWorkConfig() (*agent.WorkStatusResponseWorkConfig, error) {

	projects, err := api.FetchProjects(nil)
	if err != nil {
		return nil, err
	}
	rawstates := make(map[string]workConfigClean)
	for _, project := range projects {
		var res []workConfigRes
		url := fmt.Sprintf(`%s/_apis/wit/workitemtypes`, project.RefID)
		if err = api.getRequest(url, stringmap{}, &res); err != nil {
			return nil, err
		}
		for _, r := range res {
			var conf workConfigClean
			var ok bool
			if conf, ok = rawstates[r.Name]; !ok {
				conf = workConfigClean{
					RefName: r.ReferenceName,
					States:  make(map[string]string),
				}
			}
			for _, s := range r.States {
				conf.States[s.Name] = s.Category
			}
			rawstates[r.Name] = conf
		}
	}
	ws := &agent.WorkStatusResponseWorkConfig{}
	if _, ok := rawstates["Epic"]; ok {
		ws.TopLevelIssue = agent.WorkStatusResponseWorkConfigTopLevelIssue{
			Name: "Epic",
			Type: rawstates["Epic"].RefName,
		}
	}
	var enhancementRule agent.WorkStatusResponseWorkConfigTypeRules
	bugRule := make([]agent.WorkStatusResponseWorkConfigTypeRules, len(rawstates))
	var featureRule agent.WorkStatusResponseWorkConfigTypeRules
	var otherRule agent.WorkStatusResponseWorkConfigTypeRules
	var i int
	for _, r := range rawstates {
		ws.Types = append(ws.Types, r.RefName)
		for name, cat := range r.States {
			if cat == workConfigCompletedStatus || cat == workConfigRemovedStatus || cat == workConfigResolvedStatus {
				if cat != workConfigRemovedStatus {
					ws.Resolutions.WorkDone = appendString(ws.Resolutions.WorkDone, name)
				} else {
					ws.Resolutions.NoWorkDone = appendString(ws.Resolutions.NoWorkDone, name)
				}
				ws.AllResolutions = appendString(ws.AllResolutions, name)
			} else {
				ws.AllStatuses = appendString(ws.AllStatuses, name)
			}

			if cat == workConfigProposedStatus {
				ws.Statuses.OpenStatus = appendString(ws.Statuses.OpenStatus, name)
			}
			if cat == workConfigInProgressStatus {
				ws.Statuses.InProgressStatus = appendString(ws.Statuses.InProgressStatus, name)
			}
			if cat == workConfigRemovedStatus {
				ws.Statuses.ClosedStatus = appendString(ws.Statuses.ClosedStatus, name)
			}
			if cat == workConfigCompletedStatus {
				ws.Statuses.ReleasedStatus = appendString(ws.Statuses.ReleasedStatus, name)
			}
		}

		predicate := agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
			Field:    agent.WorkStatusResponseWorkConfigTypeRulesPredicatesFieldType,
			Operator: agent.WorkStatusResponseWorkConfigTypeRulesPredicatesOperatorEquals,
			Value:    pstrings.Pointer(r.RefName),
		}
		if stringEquals(r.RefName,
			"Microsoft.VSTS.WorkItemTypes.Issue",
			"Microsoft.VSTS.WorkItemTypes.Bug",
			"Issue", "Bug") {
			bugRule[i].IssueType = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeBug
			bugRule[i].Predicates = append(bugRule[i].Predicates, predicate)
		} else if stringEquals(r.RefName,
			"Microsoft.VSTS.WorkItemTypes.Task",
			"Task") {
			enhancementRule.IssueType = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeEnhancement
			enhancementRule.Predicates = append(enhancementRule.Predicates, predicate)
		} else if stringEquals(r.RefName,
			"Microsoft.VSTS.WorkItemTypes.FeedbackRequest",
			"Microsoft.VSTS.WorkItemTypes.Feature",
			"Feature", "Feedback Request") {
			featureRule.IssueType = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeFeature
			featureRule.Predicates = append(featureRule.Predicates, predicate)
		} else {
			otherRule.IssueType = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeOther
			otherRule.Predicates = append(otherRule.Predicates, predicate)
		}
		i++
	}
	ws.TypeRules = []agent.WorkStatusResponseWorkConfigTypeRules{enhancementRule, featureRule, otherRule}
	ws.TypeRules = append(ws.TypeRules, bugRule...)
	return ws, nil
}
