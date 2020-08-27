package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/ids2"
	pnumbers "github.com/pinpt/go-common/v10/number"
	"github.com/pinpt/integration-sdk/work"
)

const whereDateFormat = `01/02/2006 15:04:05Z`

func (api *API) FetchItemIDs(projid string, fromdate time.Time) ([]string, error) {
	return api.fetchItemIDs(projid, fromdate)
}

func (api *API) fetchItemIDs(projid string, fromdate time.Time) ([]string, error) {
	url := fmt.Sprintf(`%s/_apis/wit/wiql`, projid)
	var q struct {
		Query string `json:"query"`
	}
	q.Query = `Select System.ID From WorkItems`
	if !fromdate.IsZero() {
		q.Query += fmt.Sprintf(` WHERE System.ChangedDate > '%s'`, fromdate.Format(whereDateFormat))
	}
	var res workItemsResponse
	if err := api.postRequest(url, stringmap{"timePrecision": "true"}, q, &res); err != nil {
		return nil, err
	}
	if len(res.WorkItems) == 0 {
		return []string{}, nil
	}
	var all []string
	for _, wi := range res.WorkItems {
		all = append(all, fmt.Sprintf("%d", wi.ID))
	}
	return all, nil
}

var pullRequestFromIssue = regexp.MustCompile(`PullRequestId\/(.*?)%2F(.*?)%2F(.*?)$`)

// FetchWorkItemsByIDs used by onboard and export
func (api *API) FetchWorkItemsByIDs(projid string, ids []string) ([]WorkItemResponse, []*work.Issue, error) {
	url := fmt.Sprintf(`%s/_apis/wit/workitems?ids=%s`, projid, strings.Join(ids, ","))
	var err error
	var res []WorkItemResponse
	if err = api.getRequest(url, stringmap{"pagingoff": "true", "$expand": "all"}, &res); err != nil {
		return nil, nil, err
	}
	var res2 []*work.Issue
	for _, each := range res {
		fields := each.Fields

		if stringEquals(fields.WorkItemType,
			"Microsoft.VSTS.WorkItemTypes.SharedParameter", "SharedParameter", "Shared Parameter",
			"Microsoft.VSTS.WorkItemTypes.SharedStep", "SharedStep", "Shared Step",
			"Microsoft.VSTS.WorkItemTypes.TestCase", "TestCase", "Test Case",
			"Microsoft.VSTS.WorkItemTypes.TestPlan", "TestPlan", "Test Plan",
			"Microsoft.VSTS.WorkItemTypes.TestSuite", "TestSuite", "Test Suite",
		) {
			continue
		}

		if !api.hasResolution(projid, fields.WorkItemType) {
			if api.completedState(projid, fields.WorkItemType, fields.State) {
				fields.ResolvedReason = fields.Reason
			}
		}

		issue, err := azureIssueToPinpointIssue(each, projid, api.customerid, api.reftype, api.IDs)
		var updatedDate time.Time
		if issue.ChangeLog, updatedDate, err = api.fetchChangeLog(fields.WorkItemType, projid, issue.RefID); err != nil {
			return nil, nil, err
		}
		// this should only happen if the changelog is empty, which should never happen anyway,
		// but just in case...
		if updatedDate.IsZero() {
			updatedDate = fields.ChangedDate
		}
		date.ConvertToModel(updatedDate, &issue.UpdatedDate)

		res2 = append(res2, &issue)
	}
	return res, res2, nil
}

var completedStates map[string]string

func init() {
	completedStates = make(map[string]string)
}

func (api *API) completedState(projid string, itemtype string, state string) bool {

	if s, o := completedStates[itemtype]; o {
		return state == s
	}
	var res []workConfigRes
	url := fmt.Sprintf(`%s/_apis/wit/workitemtypes/%s`, projid, itemtype)
	if err := api.getRequest(url, stringmap{}, &res); err != nil {
		return false
	}

	conf := res[0]
	for _, r := range conf.States {
		if workConfigStatus(r.Category) == workConfigCompletedStatus {
			completedStates[itemtype] = r.Name
			return state == r.Name
		}
	}
	return false
}

func azureIssueToPinpointIssue(item WorkItemResponse, projid string, customerid string, reftype string, idgen ids2.Gen) (work.Issue, error) {

	fields := item.Fields
	issue := work.Issue{
		AssigneeRefID: fields.AssignedTo.ID,
		CreatorRefID:  fields.CreatedBy.ID,
		CustomerID:    customerid,
		Description:   fields.Description,
		Identifier:    fmt.Sprintf("%s-%d", fields.TeamProject, item.ID),
		Priority:      fmt.Sprintf("%d", fields.Priority),
		ProjectID:     idgen.WorkProject(projid),
		RefID:         fmt.Sprintf("%d", item.ID),
		RefType:       reftype,
		ReporterRefID: fields.CreatedBy.ID,
		Resolution:    itemStateName(fields.ResolvedReason, item.Fields.WorkItemType),
		Status:        itemStateName(fields.State, item.Fields.WorkItemType),
		StoryPoints:   pnumbers.Float64Pointer(fields.StoryPoints),
		Tags:          strings.Split(fields.Tags, "; "),
		Title:         fields.Title,
		Type:          fields.WorkItemType,
		URL:           item.Links.HTML.HREF,
		SprintIds:     []string{idgen.WorkSprintID(fields.IterationPath)},
	}
	/*
		TODO: check if we can remove it from item.Relation as well
		for _, rel := range item.Relations {
			if rel.Rel == "ArtifactLink" {
				matches := pullRequestFromIssue.FindAllStringSubmatch(rel.URL, 1)
				if len(matches) > 0 {
					// proj := matches[0][1]
					repo := matches[0][2]
					refid := matches[0][3]
					prid := idgen.CodePullRequest(repo, refid)
					issue.PullRequestIds = append(issue.PullRequestIds, prid)
				}
			}
		}
	*/
	date.ConvertToModel(fields.CreatedDate, &issue.CreatedDate)
	date.ConvertToModel(fields.DueDate, &issue.DueDate)
	return issue, nil
}
