package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/date"
	pnumbers "github.com/pinpt/go-common/number"
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
	if err = api.getRequest(url, stringmap{"pagingoff": "true", "$expand": "relations"}, &res); err != nil {
		return nil, nil, err
	}
	var res2 []*work.Issue
	for _, each := range res {
		fields := each.Fields
		issue := work.Issue{
			AssigneeRefID: fields.AssignedTo.ID,
			CreatorRefID:  fields.CreatedBy.ID,
			CustomerID:    api.customerid,
			Description:   fields.Description,
			Identifier:    fmt.Sprintf("%s-%d", fields.TeamProject, each.ID),
			Priority:      fmt.Sprintf("%d", fields.Priority),
			ProjectID:     api.IDs.WorkProject(projid),
			RefID:         fmt.Sprintf("%d", each.ID),
			RefType:       api.reftype,
			ReporterRefID: fields.CreatedBy.ID,
			Resolution:    fields.ResolvedReason,
			Status:        fields.State,
			StoryPoints:   pnumbers.Float64Pointer(fields.StoryPoints),
			Tags:          strings.Split(fields.Tags, "; "),
			Title:         fields.Title,
			Type:          fields.WorkItemType,
			URL:           each.URL,
		}
		for _, rel := range each.Relations {
			if rel.Rel == "ArtifactLink" {
				matches := pullRequestFromIssue.FindAllStringSubmatch(rel.URL, 1)
				if len(matches) > 0 {
					// proj := matches[0][1]
					repo := matches[0][2]
					refid := matches[0][3]
					prid := api.IDs.CodePullRequest(repo, refid)
					issue.PullRequestIds = append(issue.PullRequestIds, prid)
				}
			}
		}
		var updatedDate time.Time
		if issue.ChangeLog, updatedDate, err = api.fetchChangeLog(fields.WorkItemType, projid, issue.RefID); err != nil {
			return nil, nil, err
		}
		// this should only happen if the changelog is empty, which should never happen anyway,
		// but just in case...
		if updatedDate.IsZero() {
			updatedDate = fields.ChangedDate
		}

		date.ConvertToModel(fields.CreatedDate, &issue.CreatedDate)
		date.ConvertToModel(fields.DueDate, &issue.DueDate)
		date.ConvertToModel(updatedDate, &issue.UpdatedDate)

		res2 = append(res2, &issue)
	}
	return res, res2, nil
}
