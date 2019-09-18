package azureapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/work"
)

type changelogField struct {
	NewValue interface{} `json:"newValue"`
	OldValue interface{} `json:"oldvalue"`
}
type changelogResponse struct {
	Fields      map[string]changelogField `json:"fields"`
	ID          int64                     `json:"id"`
	RevisedDate time.Time                 `json:"revisedDate"`
	URL         string                    `json:"url"`
	Relations   struct {
		Added []struct {
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
			URL string `json:"url"`
		} `json:"added"`
		Removed []struct {
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
			URL string `json:"url"`
		} `json:"removed"`
	} `json:"relations"`
	RevisedBy usersResponse `json:"revisedBy"`
}

func (api *API) FetchChangelogs2(projid string, issueid string) error {
	var res []interface{}
	url := fmt.Sprintf(`%s/_apis/wit/workItems/%s/updates`, projid, issueid)
	err := api.getRequest(url, nil, &res)
	fmt.Println(stringify(res))
	os.Exit(1)
	return err
}

type WorkChangelogsResult struct {
	ProjectID  string
	Changelogs chan work.Changelog
}

func (api *API) FetchChangelogs(projid string, fromdate time.Time, result chan<- datamodel.Model) error {

	async := NewAsync(5)
	allids, err := api.fetchItemIDS(projid, fromdate)
	if err != nil {
		api.logger.Error("error fetching item ids", "err", err)
	}
	for _, refid := range allids {
		async.Send(AsyncMessage{
			Data: refid,
			Func: func(data interface{}) {
				refid := data.(string)
				if err := api.fetchChangeLog(projid, refid, result); err != nil {
					api.logger.Error("error fetching work item updates "+refid, "err", err)
				}
			},
		})
	}
	async.Wait()
	return nil
}

func (api *API) fetchChangeLog(projid, refid string, result chan<- datamodel.Model) error {
	var res []changelogResponse
	url := fmt.Sprintf(`%s/_apis/wit/workItems/%s/updates`, projid, refid)
	if err := api.getRequest(url, stringmap{"$top": "200"}, &res); err != nil {
		return err
	}
	issueid := api.IssueID(refid)
	for i, changelog := range res {
		if changelog.Fields == nil {
			continue
		}
		createParentField(&changelog)
		createdDate := extractCreatedDate(changelog)
		for field, values := range changelog.Fields {
			if extractor, ok := fields[field]; ok {
				name, from, to := extractor(values)
				if from == "" && to == "" {
					continue
				}
				result <- &work.Changelog{
					CreatedDate: createdDate,
					CustomerID:  api.customerid,
					Field:       name,
					FieldType:   api.reftype,
					From:        from,
					IssueID:     issueid,
					Ordinal:     int64(i),
					ProjectID:   api.ProjectID(projid),
					RefID:       fmt.Sprintf("%d", changelog.ID),
					RefType:     api.reftype,
					To:          to,
					UserID:      changelog.RevisedBy.ID,
				}
			}
		}
	}
	return nil
}

func extractCreatedDate(changelog changelogResponse) work.ChangelogCreatedDate {
	var createdDate work.ChangelogCreatedDate
	if field, ok := changelog.Fields["System.CreatedDate"]; ok {
		created, err := time.Parse(time.RFC3339, fmt.Sprint(field.NewValue))
		if err == nil {
			date.ConvertToModel(created, &createdDate)
		}
	} else {
		date.ConvertToModel(changelog.RevisedDate, &createdDate)
	}
	if createdDate.Epoch < 0 {
		return work.ChangelogCreatedDate{}
	}
	return createdDate
}

func createParentField(changelog *changelogResponse) {
	if added := changelog.Relations.Added; added != nil {
		for _, each := range added {
			if each.Attributes.Name == "Parent" {
				changelog.Fields["parent"] = changelogField{
					NewValue: filepath.Base(each.URL), // get the work item id from the url
				}
				break
			}
		}
	}
	if removed := changelog.Relations.Removed; removed != nil {
		for _, each := range removed {
			if each.Attributes.Name == "Parent" {
				var field changelogField
				var ok bool
				if field, ok = changelog.Fields["parent"]; ok {
					field.OldValue = filepath.Base(each.URL) // get the work item id from the url
				} else {
					field = changelogField{
						OldValue: filepath.Base(each.URL), // get the work item id from the url
					}
				}
				changelog.Fields["parent"] = field
				break
			}
		}
	}
}

func toString(i interface{}) string {
	if i == nil {
		return ""
	}
	if s, o := i.(string); o {
		return s
	}
	if s, o := i.(float64); o {
		return fmt.Sprintf("%f", s)
	}
	return fmt.Sprintf("%v", i)
}

type fieldExtractor func(item changelogField) (name string, from string, to string)

func extractUsers(item changelogField) (from string, to string) {
	if item.OldValue != nil {
		var user usersResponse
		structmarshal.StructToObject(item.OldValue, &user)
		from = user.ID
	}
	if item.NewValue != nil {
		var user usersResponse
		structmarshal.StructToObject(item.NewValue, &user)
		to = user.ID
	}
	return
}

var fields = map[string]fieldExtractor{
	"System.State": func(item changelogField) (string, string, string) {
		return "status", toString(item.OldValue), toString(item.NewValue)
	},
	"Microsoft.VSTS.Common.ResolvedReason": func(item changelogField) (string, string, string) {
		return "resolution", toString(item.OldValue), toString(item.NewValue)
	},
	"System.AssignedTo": func(item changelogField) (string, string, string) {
		from, to := extractUsers(item)
		return "assignee_ref_id", from, to
	},
	"System.CreatedBy": func(item changelogField) (string, string, string) {
		from, to := extractUsers(item)
		return "reporter_ref_id", from, to
	},
	"System.Title": func(item changelogField) (string, string, string) {
		return "title", toString(item.OldValue), toString(item.NewValue)
	},
	// convert to date
	"Microsoft.VSTS.Scheduling.DueDate": func(item changelogField) (string, string, string) {
		return "due_date", toString(item.OldValue), toString(item.NewValue)
	},
	"System.WorkItemType": func(item changelogField) (string, string, string) {
		return "type", toString(item.OldValue), toString(item.NewValue)
	},
	"System.Tags": func(item changelogField) (string, string, string) {
		from := toString(item.OldValue)
		to := toString(item.NewValue)
		if from != "" {
			tmp := strings.Split(from, "; ")
			from = strings.Join(tmp, ",")
		}
		if to != "" {
			tmp := strings.Split(from, "; ")
			to = strings.Join(tmp, ",")
		}
		return "tags", from, to
	},
	"Microsoft.VSTS.Common.Priority": func(item changelogField) (string, string, string) {
		return "priority", toString(item.OldValue), toString(item.NewValue)
	},
	"System.Id": func(item changelogField) (string, string, string) {
		return "project_id", toString(item.OldValue), toString(item.NewValue)
	},
	"System.TeamProject": func(item changelogField) (string, string, string) {
		return "identifier", toString(item.OldValue), toString(item.NewValue)
	},
	"System.IterationId": func(item changelogField) (string, string, string) {
		return "sprint_ids", toString(item.OldValue), toString(item.NewValue)
	},
	"parent": func(item changelogField) (string, string, string) {
		var from, to string
		if toString(item.OldValue) != "" {
			from = hash.Values("Issue", "item.CustomerID", "jira", item.OldValue)
		}
		if toString(item.NewValue) != "" {
			to = hash.Values("Issue", "item.CustomerID", "jira", item.NewValue)
		}
		return "parent_id", from, to
	},
	// "Epic Link": func(item work.Changelog) (string, interface{}, interface{}) {
	// 	var from, to string
	// 	if item.From != "" {
	// 		from = pw.NewIssueID(item.CustomerID, "jira", item.From)
	// 	}
	// 	if item.To != "" {
	// 		to = pw.NewIssueID(item.CustomerID, "jira", item.To)
	// 	}
	// 	return "epic_id", from, to
	// },
}
