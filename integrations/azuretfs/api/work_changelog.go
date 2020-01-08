package api

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pinpt/go-common/datetime"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/integration-sdk/work"
)

func (api *API) fetchChangeLog(itemtype, projid, issueid string) (changelogs []work.IssueChangeLog, latestChange time.Time, err error) {
	var res []changelogResponse
	url := fmt.Sprintf(`%s/_apis/wit/workItems/%s/updates`, projid, issueid)
	if err := api.getRequest(url, stringmap{"$top": "200"}, &res); err != nil {
		return nil, time.Time{}, err
	}
	if len(res) == 0 {
		return
	}
	previousState := ""
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

				if i == 0 && changelogToString(values.OldValue) == "" {
					continue
				}
				newvals := changelogFieldWithIDGen{
					changelogField: values,
					gen:            api.IDs,
				}
				name, from, to := extractor(newvals)
				if from == "" && to == "" {
					continue
				}

				if name == work.IssueChangeLogFieldStatus {
					if to == "" {
						previousState = from
						continue
					}
					if from == "" && previousState != "" {
						from = previousState
						previousState = ""
					}
					if to == from {
						continue
					}
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
	sort.Slice(changelogs, func(i int, j int) bool {
		return changelogs[i].CreatedDate.Epoch < changelogs[j].CreatedDate.Epoch
	})
	if len(changelogs) > 0 {
		last := changelogs[len(changelogs)-1]
		latestChange = datetime.DateFromEpoch(last.CreatedDate.Epoch)
	}
	return
}

func changeLogExtractCreatedDate(changelog changelogResponse) work.IssueChangeLogCreatedDate {
	var createdDate work.IssueChangeLogCreatedDate
	// This field is always there
	// System.ChangedDate is the created date if there is only one changelog
	if field, ok := changelog.Fields["System.ChangedDate"]; ok {
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

type changeLogFieldExtractor func(item changelogFieldWithIDGen) (name work.IssueChangeLogField, from string, to string)

func extractUsers(item changelogFieldWithIDGen) (from string, to string) {
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

var changelogFields = map[string]changeLogFieldExtractor{
	// "System.State": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
	"System.BoardColumn": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldStatus, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"Microsoft.VSTS.Common.ResolvedReason": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldResolution, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.AssignedTo": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		from, to := extractUsers(item)
		return work.IssueChangeLogFieldAssigneeRefID, from, to
	},
	"System.CreatedBy": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		from, to := extractUsers(item)
		return work.IssueChangeLogFieldReporterRefID, from, to
	},
	"System.Title": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldTitle, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	// convert to date
	"Microsoft.VSTS.Scheduling.DueDate": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldDueDate, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.WorkItemType": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldType, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.Tags": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
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
	"Microsoft.VSTS.Common.Priority": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		return work.IssueChangeLogFieldPriority, changelogToString(item.OldValue), changelogToString(item.NewValue)
	},
	"System.Id": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		oldvalue := item.gen.WorkIssue(changelogToString(item.OldValue))
		newvalue := item.gen.WorkIssue(changelogToString(item.NewValue))
		return work.IssueChangeLogFieldProjectID, oldvalue, newvalue
	},
	"System.TeamProject": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		oldvalue := item.gen.WorkProject(changelogToString(item.OldValue))
		newvalue := item.gen.WorkProject(changelogToString(item.NewValue))
		return work.IssueChangeLogFieldIdentifier, oldvalue, newvalue
	},
	"System.IterationPath": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		oldvalue := item.gen.WorkSprintID(changelogToString(item.OldValue))
		newvalue := item.gen.WorkSprintID(changelogToString(item.NewValue))
		return work.IssueChangeLogFieldSprintIds, oldvalue, newvalue
	},
	"parent": func(item changelogFieldWithIDGen) (work.IssueChangeLogField, string, string) {
		oldvalue := item.gen.WorkIssue(changelogToString(item.OldValue))
		newvalue := item.gen.WorkIssue(changelogToString(item.NewValue))
		return work.IssueChangeLogFieldParentID, oldvalue, newvalue
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
