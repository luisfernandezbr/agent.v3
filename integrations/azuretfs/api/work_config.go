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

type workConfigStatus string

// These seem to be the default statuses
const workConfigCompletedStatus = workConfigStatus("Completed")
const workConfigInProgressStatus = workConfigStatus("InProgress")
const workConfigProposedStatus = workConfigStatus("Proposed")
const workConfigRemovedStatus = workConfigStatus("Removed")
const workConfigResolvedStatus = workConfigStatus("Resolved")

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

type resolutionRes struct {
	AllowedValues []string `json:"allowedValues"`
	Name          string   `json:"name"`
	ReferenceName string   `json:"referenceName"`
}

func (api *API) FetchWorkConfig() (*agent.WorkStatusResponseWorkConfig, error) {
	projects, err := api.FetchProjects(nil)
	if err != nil {
		return nil, err
	}
	ws := &agent.WorkStatusResponseWorkConfig{}
	for _, project := range projects {

		api.fetchColumnsForStatuses(project.RefID, ws)

		var res []workConfigRes

		url := fmt.Sprintf(`%s/_apis/wit/workitemtypes`, project.RefID)
		if err = api.getRequest(url, stringmap{}, &res); err != nil {
			return nil, err
		}
		for _, r := range res {

			if r.Name == "Epic" {
				ws.TopLevelIssue = agent.WorkStatusResponseWorkConfigTopLevelIssue{
					Name: "Epic",
					Type: "Epic",
				}
			}

			// for _, s := range r.States {
			// 	name := itemStateName(s.Name, r.Name)
			// 	switch workConfigStatus(s.Category) {
			// 	case workConfigCompletedStatus:
			// 		// Work items whose state is in this category don't appear on the backlog
			// 		ws.Statuses.ReleasedStatus = appendString(ws.Statuses.ReleasedStatus, name)
			// 	case workConfigInProgressStatus:
			// 		// Assigned to states that represent active work
			// 		ws.Statuses.InProgressStatus = appendString(ws.Statuses.InProgressStatus, name)
			// 	case workConfigProposedStatus:
			// 		// Assigned to states associated with newly added work items so that they appear on the backlog
			// 		ws.Statuses.OpenStatus = appendString(ws.Statuses.OpenStatus, name)
			// 	case workConfigRemovedStatus:
			// 		// Work items in a state mapped to the Removed category are hidden from the backlog and board experiences
			// 		ws.Statuses.ClosedStatus = appendString(ws.Statuses.ClosedStatus, name)
			// 	case workConfigResolvedStatus:
			// 		// Assigned to states that represent a solution has been implemented, but are not yet verified
			// 		ws.Statuses.InReviewStatus = appendString(ws.Statuses.InReviewStatus, name)
			// 	}
			// 	ws.AllStatuses = appendString(ws.AllStatuses, name)
			// }

			url2 := fmt.Sprintf(`%s/_apis/wit/workitemtypes/%s/fields`, project.RefID, r.ReferenceName)
			var resres []resolutionRes
			if err := api.getRequest(url2, stringmap{"$expand": "allowedValues"}, &resres); err != nil {
				return nil, err
			}
			for _, g := range resres {
				if len(g.AllowedValues) > 0 {
					if g.ReferenceName == "Microsoft.VSTS.Common.ResolvedReason" {
						for _, n := range g.AllowedValues {
							ws.AllResolutions = appendString(ws.AllResolutions, n)
						}
					}
					// 	continue
					// }
					// if g.ReferenceName == "System.BoardColumn" {
					// 	for _, name := range g.AllowedValues {
					// 		switch name {
					// 		case "Fixed", "Abandoned", "Issue Removed", "Issue Resolved":
					// 			ws.Statuses.ClosedStatus = appendString(ws.Statuses.ClosedStatus, name)
					// 		case "New":
					// 			ws.Statuses.OpenStatus = appendString(ws.Statuses.OpenStatus, name)
					// 		case "Not fixed", "Reactivated":
					// 			ws.Statuses.ReopenedStatus = appendString(ws.Statuses.ReopenedStatus, name)
					// 		case "Implementation started", "Work started":
					// 			ws.Statuses.InProgressStatus = appendString(ws.Statuses.InProgressStatus, name)
					// 		case "Closed", "Completed":
					// 			ws.Statuses.ReleasedStatus = appendString(ws.Statuses.ReleasedStatus, name)
					// 		}
					// 		ws.AllStatuses = appendString(ws.AllStatuses, name)
					// 	}
					// 	continue
					// }
				}
			}

			// type boardcolumnsRes struct {
			// 	Name string `json:"name"`
			// }
			// url3 := fmt.Sprintf(`%s/_apis/work/boardcolumns`, project.RefID)
			// var resres2 []boardcolumnsRes
			// if err := api.getRequest(url3, nil, &resres2); err != nil {
			// 	return nil, err
			// }
			// for _, g := range resres2 {
			// 				ws.AllResolutions = appendString(ws.AllResolutions, g.Name)
			// 				switch g.Name {
			// 				case "":
			// 					break
			// 				}
			// }

			predicate := agent.WorkStatusResponseWorkConfigTypeRulesPredicates{
				Field:    predFieldType,
				Operator: predEquals,
				Value:    pstrings.Pointer(r.Name),
			}
			if stringEquals(r.ReferenceName,
				"Microsoft.VSTS.WorkItemTypes.Issue",
				"Microsoft.VSTS.WorkItemTypes.Bug",
				"Issue", "Bug") {
				ws.TypeRules = append(ws.TypeRules, agent.WorkStatusResponseWorkConfigTypeRules{
					IssueType:  typeBug,
					Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{predicate},
				})
			} else if stringEquals(r.ReferenceName,
				"Microsoft.VSTS.WorkItemTypes.Task",
				"Task") {
				ws.TypeRules = append(ws.TypeRules, agent.WorkStatusResponseWorkConfigTypeRules{
					IssueType:  typeEnhancement,
					Predicates: []agent.WorkStatusResponseWorkConfigTypeRulesPredicates{predicate},
				})
			} else if stringEquals(r.ReferenceName,
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
			ws.Types = append(ws.Types, r.Name)
		}
	}
	return ws, err
}

func (api *API) fetchColumnsForStatuses(projid string, ws *agent.WorkStatusResponseWorkConfig) {
	ids, err := api.FetchTeamIDs(projid)
	if err != nil {
		panic(err)
	}
	for _, i := range ids {
		url1 := fmt.Sprintf(`/%s/%s/_apis/work/boards`, projid, i)
		var res1 []struct {
			ID string `json:"id"`
		}
		err = api.getRequest(url1, nil, &res1)
		if err != nil {
			panic(err)
		}

		url2 := fmt.Sprintf(`/%s/%s/_apis/work/boards/%s/columns`, projid, i, res1[0].ID)
		var res2 []struct {
			ColType string `json:"columnType"`
			Name    string `json:"name"`
		}
		err = api.getRequest(url2, nil, &res2)
		if err != nil {
			panic(err)
		}
		for _, e := range res2 {
			switch e.ColType {
			case "incoming":
				if exists(e.Name, ws.Statuses.InProgressStatus) {
					api.logger.Error("status " + e.Name + " already exists in In Progress")
					break
				}
				if exists(e.Name, ws.Statuses.ClosedStatus) {
					api.logger.Error("status " + e.Name + " already exists in Closed")
					break
				}
				ws.Statuses.OpenStatus = appendString(ws.Statuses.OpenStatus, e.Name)
			case "inProgress":
				if exists(e.Name, ws.Statuses.OpenStatus) {
					api.logger.Error("status " + e.Name + " already exists in Open")
					break
				}
				if exists(e.Name, ws.Statuses.ClosedStatus) {
					api.logger.Error("status " + e.Name + " already exists in Closed")
					break
				}
				ws.Statuses.InProgressStatus = appendString(ws.Statuses.InProgressStatus, e.Name)
			case "outgoing":
				ws.Statuses.ClosedStatus = appendString(ws.Statuses.ClosedStatus, e.Name)
				if exists(e.Name, ws.Statuses.OpenStatus) {
					api.logger.Error("status " + e.Name + " already exists in Open")
					break
				}
				if exists(e.Name, ws.Statuses.InProgressStatus) {
					api.logger.Error("status " + e.Name + " already exists in In Progress")
					break
				}
			}
			ws.AllStatuses = appendString(ws.AllStatuses, e.Name)
		}
	}
}
