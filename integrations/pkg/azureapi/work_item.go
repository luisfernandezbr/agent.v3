package azureapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/integration-sdk/work"
)

type workItemOperation struct {
	Op    string  `json:"op"`
	Path  string  `json:"path"`
	From  *string `json:"from"`
	Value string  `json:"value"`
}

func (api *API) CreateWorkItems(projid string) error {

	url := fmt.Sprintf(`%s/_apis/wit/workitems/%s`, projid, `$Issue`)
	for i := 848; i < 1000; i++ {
		ops := []workItemOperation{
			workItemOperation{
				Op:    "add",
				Path:  "/fields/System.Title",
				Value: fmt.Sprintf("this is a title for issue number %d", i),
			},
			workItemOperation{
				Op:    "add",
				Path:  "/fields/System.Description",
				Value: fmt.Sprintf("this is a description for issue number %d", i),
			},
		}
		var res []interface{}
		if err := api.postRequest(url, stringmap{}, ops, &res); err != nil {
			return err
		}
		_ = res
		fmt.Println(i)
	}

	return nil
}

type workItemsResponse struct {
	AsOf    time.Time `json:"asOf"`
	Columns []struct {
		Name          string `json:"name"`
		ReferenceName string `json:"referenceName"`
		URL           string `json:"url"`
	} `json:"columns"`
	QueryResultType string `json:"queryResultType"`
	QueryType       string `json:"queryType"`
	SortColumns     []struct {
		Descending bool `json:"descending"`
		Field      struct {
			Name          string `json:"name"`
			ReferenceName string `json:"referenceName"`
			URL           string `json:"url"`
		} `json:"field"`
	} `json:"sortColumns"`
	WorkItems []struct {
		ID  int64  `json:"id"`
		URL string `json:"url"`
	} `json:"workItems"`
}

type WorkItemsResult struct {
	ProjectID string
	Issues    chan work.Issue
}

func (api *API) fetchItemIDS(projid string, fromdate time.Time) ([]string, error) {
	url := fmt.Sprintf(`%s/_apis/wit/wiql`, projid)
	var q struct {
		Query string `json:"query"`
	}
	q.Query = `Select System.ID From WorkItems`
	if !fromdate.IsZero() {
		q.Query += `  WHERE System.ChangedDate > '` + fromdate.Format("01/02/2006 15:04:05Z") + `'`
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
func (api *API) FetchWorkItems(projids []string, fromdate time.Time, items chan WorkItemsResult) error {

	type msgdata struct {
		projid string
		ids    []string
		issues chan work.Issue
	}
	for _, projid := range projids {
		async := NewAsync(5)
		// create the result channel object
		issues := make(chan work.Issue)
		items <- WorkItemsResult{
			ProjectID: projid,
			Issues:    issues,
		}
		allids, err := api.fetchItemIDS(projid, fromdate)
		if err != nil {
			return err
		}
		var ids []string
		for _, id := range allids {
			ids = append(ids, id)
			if len(ids) == 200 {
				async.Send(AsyncMessage{
					Data: msgdata{projid, ids, issues},
					F: func(data interface{}) {
						d := data.(msgdata)
						if err := api.FetchWorkItemsByIDs(d.projid, d.ids, d.issues); err != nil {
							api.logger.Error("error with FetchWorkItemsByIDs", "err", err)
						}
					},
				})
				ids = []string{}
			}
		}
		if len(ids) > 0 {
			async.Send(AsyncMessage{
				Data: msgdata{projid, ids, issues},
				F: func(data interface{}) {
					d := data.(msgdata)
					if err := api.FetchWorkItemsByIDs(d.projid, d.ids, d.issues); err != nil {
						api.logger.Error("error with FetchWorkItemsByIDs", "err", err)
					}
				},
			})
		}
		async.Wait()
		close(issues)
	}
	return nil
}

func (api *API) FetchWorkItemsByIDs(projid string, ids []string, items chan work.Issue) error {
	url := fmt.Sprintf(`%s/_apis/wit/workitems?ids=%s`, projid, strings.Join(ids, ","))
	var res []workItemResponse
	if err := api.getRequest(url, stringmap{"pagingoff": "true"}, &res); err != nil {
		return err
	}
	for _, each := range res {
		fields := each.Fields
		issue := work.Issue{
			AssigneeRefID: fields.AssignedTo.ID,
			CreatorRefID:  fields.CreatedBy.ID,
			// CustomFields:
			CustomerID: api.customerid,
			// DueDate:
			// ID:
			Identifier: fields.TeamProject, //??
			// ParentID:
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
			// UpdatedDate:
			// UpdatedAt:
			URL: each.URL,
			// Hashcode:
		}
		date.ConvertToModel(fields.CreatedDate, &issue.CreatedDate)
		date.ConvertToModel(fields.DueDate, &issue.DueDate)
		date.ConvertToModel(fields.ChangedDate, &issue.UpdatedDate)
		items <- issue
	}
	return nil
}

type workItemResponse struct {
	Fields struct {
		AssignedTo     usersResponse `json:"System.AssignedTo"`
		CreatedDate    time.Time     `json:"System.CreatedDate"`
		CreatedBy      usersResponse `json:"System.CreatedBy"`
		DueDate        time.Time     `json:"Microsoft.VSTS.Scheduling.DueDate"` // ??
		TeamProject    string        `json:"System.TeamProject"`
		Priority       int           `json:"Microsoft.VSTS.Common.Priority"`
		ResolvedReason string        `json:"Microsoft.VSTS.Common.ResolvedReason"`
		ResolvedDate   time.Time     `json:"Microsoft.VSTS.Common.ResolvedDate"`
		State          string        `json:"System.State"`
		Tags           string        `json:"System.Tags"`
		Title          string        `json:"System.Title"`
		WorkItemType   string        `json:"System.WorkItemType"`
		ChangedDate    time.Time     `json:"System.ChangedDate"`
	} `json:"fields"`
	ID  int    `json:"id"`
	URL string `json:"url"`
}
