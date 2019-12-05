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

const typeBug = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeBug
const typeFeature = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeFeature
const typeOther = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeOther
const typeEnhancement = agent.WorkStatusResponseWorkConfigTypeRulesIssueTypeEnhancement
const predFieldType = agent.WorkStatusResponseWorkConfigTypeRulesPredicatesFieldType
const predEquals = agent.WorkStatusResponseWorkConfigTypeRulesPredicatesOperatorEquals

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

	for _, r := range rawstates {
		ws.Types = append(ws.Types, r.RefName)
		for name, cat := range r.States {
			if cat == workConfigCompletedStatus || cat == workConfigResolvedStatus {
				if cat == workConfigCompletedStatus {
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
		}

		predicate := agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
			Field:    predFieldType,
			Operator: predEquals,
			Value:    pstrings.Pointer(r.RefName),
		}
		if stringEquals(r.RefName,
			"Microsoft.VSTS.WorkItemTypes.Issue",
			"Microsoft.VSTS.WorkItemTypes.Bug",
			"Issue", "Bug") {
			ws.TypeRules = append(ws.TypeRules, agent.WorkStatusResponseWorkConfigTypeRules{
				IssueType:  typeBug,
				Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{predicate},
			})
		} else if stringEquals(r.RefName,
			"Microsoft.VSTS.WorkItemTypes.Task",
			"Task") {
			ws.TypeRules = append(ws.TypeRules, agent.WorkStatusResponseWorkConfigTypeRules{
				IssueType:  typeEnhancement,
				Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{predicate},
			})
		} else if stringEquals(r.RefName,
			"Microsoft.VSTS.WorkItemTypes.FeedbackRequest",
			"Microsoft.VSTS.WorkItemTypes.Feature",
			"Feature", "Feedback Request") {
			ws.TypeRules = append(ws.TypeRules, agent.WorkStatusResponseWorkConfigTypeRules{
				IssueType:  typeFeature,
				Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{predicate},
			})
		} else {
			ws.TypeRules = append(ws.TypeRules, agent.WorkStatusResponseWorkConfigTypeRules{
				IssueType:  typeOther,
				Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{predicate},
			})
		}
	}
	return ws, nil
}
