package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/work"
)

const whereDateFormat = `01/02/2006 15:04:05Z`

// FetchWorkItems gets the work items (issues) and sends them to the items channel
// The first step is to get the IDs of all items that changed after the fromdate
// Then we need to get the items 200 at a time, this is done async
func (api *API) FetchWorkItems(projid string, fromdate time.Time, items chan<- datamodel.Model) error {
	async := NewAsync(api.concurrency)
	allids, err := api.fetchItemIDs(projid, fromdate)
	if err != nil {
		return err
	}
	fetchitems := func(ids []string) {
		async.Do(func() {
			if _, err := api.fetchWorkItemsByIDs(projid, ids, items); err != nil {
				api.logger.Error("error with fetchWorkItemsByIDs", "err", err)
			}
		})
	}
	var ids []string
	for _, id := range allids {
		ids = append(ids, id)
		if len(ids) == 200 {
			fetchitems(ids)
			ids = []string{}
		}
	}
	if len(ids) > 0 {
		fetchitems(ids)
	}
	async.Wait()
	return nil
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

func (api *API) fetchWorkItemsByIDs(projid string, ids []string, items chan<- datamodel.Model) ([]workItemResponse, error) {
	url := fmt.Sprintf(`%s/_apis/wit/workitems?ids=%s`, projid, strings.Join(ids, ","))
	var res []workItemResponse
	if err := api.getRequest(url, stringmap{"pagingoff": "true"}, &res); err != nil {
		return nil, err
	}
	for _, each := range res {
		fields := each.Fields
		issue := work.Issue{
			AssigneeRefID: fields.AssignedTo.ID,
			CreatorRefID:  fields.CreatedBy.ID,
			CustomerID:    api.customerid,
			Identifier:    fmt.Sprintf("%s-%d", fields.TeamProject, each.ID),
			Priority:      fmt.Sprintf("%d", fields.Priority),
			ProjectID:     projid,
			RefID:         fmt.Sprintf("%d", each.ID),
			RefType:       api.reftype,
			ReporterRefID: fields.CreatedBy.ID,
			Resolution:    fields.ResolvedReason,
			Status:        fields.State,
			Tags:          strings.Split(fields.Tags, "; "),
			Title:         fields.Title,
			Type:          fields.WorkItemType,
			URL:           each.URL,
		}
		date.ConvertToModel(fields.CreatedDate, &issue.CreatedDate)
		date.ConvertToModel(fields.DueDate, &issue.DueDate)
		date.ConvertToModel(fields.ChangedDate, &issue.UpdatedDate)
		items <- &issue
	}
	return res, nil
}
