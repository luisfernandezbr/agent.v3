package api

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/work"
)

func (api *API) fetchChangeLog(projid, issueid string) (changelogs []work.IssueChangeLog, err error) {
	var res []changelogResponse
	url := fmt.Sprintf(`%s/_apis/wit/workItems/%s/updates`, projid, issueid)
	if err := api.getRequest(url, stringmap{"$top": "200"}, &res); err != nil {
		return nil, err
	}
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
				changelogs = append(changelogs, work.IssueChangeLog{
					RefID:       fmt.Sprintf("%d", changelog.ID),
					CreatedDate: createdDate,
					Field:       name,
					From:        from,
					FromString:  from,
					Ordinal:     int64(i),
					To:          to,
					ToString:    to,
					UserID:      changelog.RevisedBy.ID,
				})
			}
		}
	}
	return
}

func changeLogExtractCreatedDate(changelog changelogResponse) work.IssueChangeLogCreatedDate {
	var createdDate work.IssueChangeLogCreatedDate
	if field, ok := changelog.Fields["System.CreatedDate"]; ok {
		created, err := time.Parse(time.RFC3339, fmt.Sprint(field.NewValue))
		if err == nil {
			date.ConvertToModel(created, &createdDate)
		}
	} else {
		date.ConvertToModel(changelog.RevisedDate, &createdDate)
	}
	if createdDate.Epoch < 0 {
		return work.IssueChangeLogCreatedDate{}
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

type changeogFieldExtractor func(item changelogField) (name work.IssueChangeLogField, from string, to string)

func extractUsers(item changelogField) (from string, to string) {
	if item.OldValue != nil {
		var user usersResponse
		structmarshal.StructToStruct(item.OldValue, &user)
		from = user.ID
	}
	if item.NewValue != nil {
		var user usersResponse
		structmarshal.StructToStruct(item.NewValue, &user)
		to = user.ID
	}
	return
}

var changelogFields = map[string]changeogFieldExtractor{
	"System.State": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldStatus, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"Microsoft.VSTS.Common.ResolvedReason": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldResolution, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.AssignedTo": func(item changelogField) (work.IssueChangeLogField, string, string) {
		from, to := extractUsers(item)
		return work.IssueChangeLogFieldAssigneeRefID, from, to
	},
	"System.CreatedBy": func(item changelogField) (work.IssueChangeLogField, string, string) {
		from, to := extractUsers(item)
		return work.IssueChangeLogFieldReporterRefID, from, to
	},
	"System.Title": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldTitle, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	// convert to date
	"Microsoft.VSTS.Scheduling.DueDate": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldDueDate, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.WorkItemType": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldType, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.Tags": func(item changelogField) (work.IssueChangeLogField, string, string) {
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
		return work.IssueChangeLogFieldTags, from, to
	},
	"Microsoft.VSTS.Common.Priority": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldPriority, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.Id": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldProjectID, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.TeamProject": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldIdentifier, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.IterationId": func(item changelogField) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldSprintIds, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"parent": func(item changelogField) (work.IssueChangeLogField, string, string) {
		var from, to string
		if changelogToString(item.OldValue) != "" {
			from = hash.Values("Issue", "item.CustomerID", "jira", item.OldValue)
		}
		if changelogToString(item.NewValue) != "" {
			to = hash.Values("Issue", "item.CustomerID", "jira", item.NewValue)
		}
		return work.IssueChangeLogFieldParentID, from, to
	},
	// "Epic Link": func(item work.IssueChangeLog) (string, interface{}, interface{}) {
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
