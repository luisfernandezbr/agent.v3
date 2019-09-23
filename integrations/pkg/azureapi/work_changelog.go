package azureapi

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/work"
)

// FetchChangelogs gets the changelogs for a single project and sends them to the result channel
// First we need to get the IDs of the items that hav changed after the fromdate
// Then we need to get each changelog individually.
func (api *API) FetchChangelogs(projid string, fromdate time.Time, result chan<- datamodel.Model) error {
	async := NewAsync(api.concurrency)
	allids, err := api.fetchItemIDs(projid, fromdate)
	if err != nil {
		api.logger.Error("error fetching item ids", "err", err)
	}
	for _, refid := range allids {
		refid := refid
		async.Send(func() {
			if _, err := api.fetchChangeLog(projid, refid, result); err != nil {
				api.logger.Error("error fetching work item updates "+refid, "err", err)
			}
		})
	}
	async.Wait()
	return nil
}

func (api *API) fetchChangeLog(projid, refid string, result chan<- datamodel.Model) ([]changelogResponse, error) {
	var res []changelogResponse
	url := fmt.Sprintf(`%s/_apis/wit/workItems/%s/updates`, projid, refid)
	if err := api.getRequest(url, stringmap{"$top": "200"}, &res); err != nil {
		return nil, err
	}
	issueid := api.IssueID(refid)
	for i, changelog := range res {
		if changelog.Fields == nil {
			continue
		}
		// check if there is a parent
		changelogCreateParentField(&changelog)
		// get the created date, if any. Some changelogs don't have this
		createdDate := changeLogExtractCreatedDate(changelog)
		for field, values := range changelog.Fields {
			if extractor, ok := changelogFields[field]; ok {
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
	return res, nil
}

func changeLogExtractCreatedDate(changelog changelogResponse) work.ChangelogCreatedDate {
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

func changelogCreateParentField(changelog *changelogResponse) {
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

func changelogToString(i interface{}) string {
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

type changeogFieldExtractor func(item changelogField) (name string, from string, to string)

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

var changelogFields = map[string]changeogFieldExtractor{
	"System.State": func(item changelogField) (string, string, string) {
		return "status", changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"Microsoft.VSTS.Common.ResolvedReason": func(item changelogField) (string, string, string) {
		return "resolution", changelogToString(item.OldValue), changelogToString(item.NewValue)
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
		return "title", changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	// convert to date
	"Microsoft.VSTS.Scheduling.DueDate": func(item changelogField) (string, string, string) {
		return "due_date", changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.WorkItemType": func(item changelogField) (string, string, string) {
		return "type", changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.Tags": func(item changelogField) (string, string, string) {
		from := changelogToString(item.OldValue)
		to := changelogToString(item.NewValue)
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
		return "priority", changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.Id": func(item changelogField) (string, string, string) {
		return "project_id", changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.TeamProject": func(item changelogField) (string, string, string) {
		return "identifier", changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.IterationId": func(item changelogField) (string, string, string) {
		return "sprint_ids", changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"parent": func(item changelogField) (string, string, string) {
		var from, to string
		if changelogToString(item.OldValue) != "" {
			from = hash.Values("Issue", "item.CustomerID", "jira", item.OldValue)
		}
		if changelogToString(item.NewValue) != "" {
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
