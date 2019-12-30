package jiracommonapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/go-common/datetime"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/work"
)

// TODO: check if all fields are needed, not that it's internal struct
type CustomField struct {
	ID    string
	Name  string
	Value string
}

type IssueWithCustomFields struct {
	*work.Issue
	CustomFields []CustomField
}

func relativeDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	if h > 0 {
		return fmt.Sprintf("-%dm", h*60+m) // convert to minutes
	}
	if m == 0 {
		return "-1m" // always return at least 1m ago
	}
	return fmt.Sprintf("-%dm", m)
}

// IssuesAndChangelogsPage returns issues and related changelogs. Calls qc.ExportUser for each user. Current difference from jira-cloud version is that user.Key is used instead of user.AccountID everywhere.
func IssuesAndChangelogsPage(
	qc QueryContext,
	project Project,
	fieldByKey map[string]*work.CustomField,
	updatedSince time.Time,
	paginationParams url.Values) (
	pi PageInfo,
	resIssues []IssueWithCustomFields,

	rerr error) {

	objectPath := "search"
	params := paginationParams

	//params.Set("maxResults", "1") // for testing
	params.Set("validateQuery", "strict")
	jql := `project="` + project.JiraID + `"`

	if !updatedSince.IsZero() {
		s := relativeDuration(time.Since(updatedSince))
		jql += fmt.Sprintf(` and (created >= "%s" or updated >= "%s")`, s, s)
	}

	// CAREFUL. pipeline right now requires specific ordering for issues
	// Only needed for pipeline. Could remove otherwise.
	jql += " ORDER BY created ASC"

	params.Set("jql", jql)
	// we need both fields and renderedFields so that we can get the unprocessed (fields) and processed (html for renderedFields)
	params.Add("expand", "changelog,fields,renderedFields")

	qc.Logger.Debug("issues request", "project", project.Key, "params", params)

	var rr struct {
		Total      int `json:"total"`
		MaxResults int `json:"maxResults"`
		Issues     []struct {
			ID             string                 `json:"id"`
			Key            string                 `json:"key"`
			Fields         map[string]interface{} `json:"fields"`
			RenderedFields struct {
				Description string `json:"description"`
			} `json:"renderedFields"`
			Changelog struct {
				Histories []struct {
					ID      string `json:"id"`
					Author  User   `json:"author"`
					Created string `json:"created"`
					Items   []struct {
						Field      string `json:"field"`
						FieldType  string `json:"fieldtype"`
						From       string `json:"from"`
						FromString string `json:"fromString"`
						To         string `json:"to"`
						ToString   string `json:"toString"`
					} `json:"items"`
				} `json:"histories"`
			} `json:"changelog"`
		} `json:"issues"`
	}

	err := qc.Request(objectPath, params, &rr)
	if err != nil {
		rerr = err
		return
	}

	var imgRegexp = regexp.MustCompile(`(?s)<span class="image-wrap"[^\>]*>(.*?src\=(?:\"|\')(.+?)(?:\"|\').*?)<\/span>`)

	type Fields struct {
		Summary  string `json:"summary"`
		DueDate  string `json:"duedate"`
		Created  string `json:"created"`
		Updated  string `json:"updated"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
		Resolution struct {
			Name string `json:"name"`
		} `json:"resolution"`
		Creator  User
		Reporter User
		Assignee User
		Labels   []string `json:"labels"`
	}

	var issuesTypedFields []Fields

	for _, issue := range rr.Issues {
		var f2 Fields
		err := structmarshal.MapToStruct(issue.Fields, &f2)
		if err != nil {
			rerr = err
			return
		}
		issuesTypedFields = append(issuesTypedFields, f2)
	}

	pi.Total = rr.Total
	pi.MaxResults = rr.MaxResults
	if len(rr.Issues) == rr.MaxResults {
		pi.HasMore = true
	}

	// ordinal should be a monotonically increasing number for changelogs
	// the value itself doesn't matter as long as the changelog is from
	// the oldest to the newest
	//
	// Using current timestamp here instead of int, so the number is also an increasing one when running incrementals compared to previous values in the historical.
	ordinal := datetime.EpochNow()

	for i, data := range rr.Issues {

		fields := issuesTypedFields[i]

		item := IssueWithCustomFields{}
		item.Issue = &work.Issue{}
		item.CustomerID = qc.CustomerID
		item.RefID = data.ID
		item.RefType = "jira"
		item.Identifier = data.Key
		item.ProjectID = qc.ProjectID(project.JiraID)

		if fields.DueDate != "" {
			orig := fields.DueDate
			d, err := time.ParseInLocation("2006-01-02", orig, time.UTC)
			if err != nil {
				rerr = fmt.Errorf("could not parse duedate of jira issue: %v err: %v", orig, err)
				return
			}
			date.ConvertToModel(d, &item.DueDate)
		}

		item.Title = fields.Summary
		if data.RenderedFields.Description != "" {
			// we need to pull out the HTML and parse it so we can properly display it in the application. the HTML will
			// have a bunch of stuff we need to cleanup for rendering in our application such as relative urls, etc. we
			// clean this up here and fix any urls and weird html issues
			item.Description = data.RenderedFields.Description
			for _, m := range imgRegexp.FindAllStringSubmatch(item.Description, -1) {
				url := m[2] // this is the image group
				// if a relative url but not a // meaning protocol to the page, then make an absolute url to the server
				if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "//") {
					// replace the relative url with an absolute url. the app will handle the case where the app
					// image is unreachable because the server is behind a corporate firewall and the user isn't on
					// the network when viewing it
					url = pstrings.JoinURL(qc.WebsiteURL, url)
				}
				// replace the <span> wrapped thumbnail junk with just a simple image tag
				newval := strings.Replace(m[0], m[1], `<img src="`+url+`" />`, 1)
				item.Description = strings.ReplaceAll(item.Description, m[0], newval)
			}
			// we apply a special tag here to allow the front-end to handle integration specific data for the integration in
			// case we need to do integration specific image handling
			item.Description = `<div class="source-jira">` + strings.TrimSpace(item.Description) + `</div>`
		}

		created, err := ParseTime(fields.Created)
		if err != nil {
			rerr = err
			return
		}
		date.ConvertToModel(created, &item.CreatedDate)
		updated, err := ParseTime(fields.Updated)
		if err != nil {
			rerr = err
			return
		}
		date.ConvertToModel(updated, &item.UpdatedDate)

		item.Priority = fields.Priority.Name
		item.Type = fields.IssueType.Name
		item.Status = fields.Status.Name
		item.Resolution = fields.Resolution.Name

		if !fields.Creator.IsZero() {
			item.CreatorRefID = fields.Creator.RefID()
			err := qc.ExportUser(fields.Creator)
			if err != nil {
				rerr = err
				return
			}
		}
		if !fields.Reporter.IsZero() {
			item.ReporterRefID = fields.Reporter.RefID()
			err := qc.ExportUser(fields.Reporter)
			if err != nil {
				rerr = err
				return
			}
		}
		if !fields.Assignee.IsZero() {
			item.AssigneeRefID = fields.Assignee.RefID()
			err := qc.ExportUser(fields.Assignee)
			if err != nil {
				rerr = err
				return
			}
		}

		item.URL = qc.IssueURL(data.Key)
		item.Tags = fields.Labels

		for k, d := range data.Fields {
			if !strings.HasPrefix(k, "customfield_") {
				continue
			}

			fd, ok := fieldByKey[k]
			if !ok {
				qc.Logger.Error("when processing jira issues, could not find field definition by key", "project", project.Key, "key", k)
				continue
			}
			v := ""
			if d != nil {
				ds, ok := d.(string)
				if ok {
					v = ds
				} else {
					b, err := json.Marshal(d)
					if err != nil {
						rerr = err
						return
					}
					v = string(b)
				}
			}

			if fd.Name == "Start Date" && v != "" {
				d, err := ParsePlannedDate(v)
				if err != nil {
					qc.Logger.Error("could not parse field %v as date, err: %v", fd.Name, err)
					continue
				}
				date.ConvertToModel(d, &item.PlannedStartDate)
			} else if fd.Name == "End Date" && v != "" {
				d, err := ParsePlannedDate(v)
				if err != nil {
					qc.Logger.Error("could not parse field %v as date, err: %v", fd.Name, err)
					continue
				}
				date.ConvertToModel(d, &item.PlannedEndDate)
			}

			f := CustomField{}
			f.ID = fd.Key
			f.Name = fd.Name
			f.Value = v
			item.CustomFields = append(item.CustomFields, f)
		}

		issueRefID := item.RefID

		issue := item

		// Jira changelog histories are ordered from the newest to the oldest but we want changelogs to be
		// sent from the oldest event to the newest event when we send
		for i := len(data.Changelog.Histories) - 1; i >= 0; i-- {
			cl := data.Changelog.Histories[i]
			for _, data := range cl.Items {

				item := work.IssueChangeLog{}
				item.RefID = cl.ID
				item.Ordinal = ordinal

				ordinal++

				createdAt, err := ParseTime(cl.Created)
				if err != nil {
					rerr = fmt.Errorf("could not parse created time of changelog for issue: %v err: %v", issueRefID, err)
					return
				}
				date.ConvertToModel(createdAt, &item.CreatedDate)
				item.UserID = cl.Author.RefID()

				item.FromString = data.FromString + " @ " + data.From
				item.ToString = data.ToString + " @ " + data.To

				switch strings.ToLower(data.Field) {
				case "status":
					item.Field = work.IssueChangeLogFieldStatus
					item.From = data.FromString
					item.To = data.ToString
				case "resolution":
					item.Field = work.IssueChangeLogFieldResolution
					item.From = data.FromString
					item.To = data.ToString
				case "assignee":
					item.Field = work.IssueChangeLogFieldAssigneeRefID
					if data.From != "" {
						item.From = ids.WorkUserAssociatedRefID(qc.CustomerID, "jira", data.From)
					}
					if data.To != "" {
						item.To = ids.WorkUserAssociatedRefID(qc.CustomerID, "jira", data.To)
					}
				case "reporter":
					item.Field = work.IssueChangeLogFieldReporterRefID
					item.From = data.From
					item.To = data.To
				case "summary":
					item.Field = work.IssueChangeLogFieldTitle
					item.From = data.FromString
					item.To = data.ToString
				case "duedate":
					item.Field = work.IssueChangeLogFieldDueDate
					item.From = data.FromString
					item.To = data.ToString
				case "issuetype":
					item.Field = work.IssueChangeLogFieldType
					item.From = data.FromString
					item.To = data.ToString
				case "labels":
					item.Field = work.IssueChangeLogFieldTags
					item.From = data.FromString
					item.To = data.ToString
				case "priority":
					item.Field = work.IssueChangeLogFieldPriority
					item.From = data.FromString
					item.To = data.ToString
				case "project":
					item.Field = work.IssueChangeLogFieldProjectID
					if data.From != "" {
						item.From = work.NewProjectID(qc.CustomerID, "jira", data.From)
					}
					if data.To != "" {
						item.To = work.NewProjectID(qc.CustomerID, "jira", data.To)
					}
				case "key":
					item.Field = work.IssueChangeLogFieldIdentifier
					item.From = data.FromString
					item.To = data.ToString
				case "sprint":
					item.Field = work.IssueChangeLogFieldSprintIds
					var from, to []string
					if data.From != "" {
						for _, s := range strings.Split(data.From, ",") {
							from = append(from, work.NewSprintID(qc.CustomerID, strings.TrimSpace(s), "jira"))
						}
					}
					if data.To != "" {
						for _, s := range strings.Split(data.To, ",") {
							to = append(to, work.NewSprintID(qc.CustomerID, strings.TrimSpace(s), "jira"))
						}
					}
					item.From = strings.Join(from, ",")
					item.To = strings.Join(to, ",")
				case "parent":
					item.Field = work.IssueChangeLogFieldParentID
					if data.From != "" {
						item.From = work.NewIssueID(qc.CustomerID, "jira", data.From)
					}
					if data.To != "" {
						item.To = work.NewIssueID(qc.CustomerID, "jira", data.To)
					}
				case "epic link":
					item.Field = work.IssueChangeLogFieldEpicID
					if data.From != "" {
						item.From = work.NewIssueID(qc.CustomerID, "jira", data.From)
					}
					if data.To != "" {
						item.To = work.NewIssueID(qc.CustomerID, "jira", data.To)
					}
				default:
					// Ignore other change types
					continue
				}
				issue.ChangeLog = append(issue.ChangeLog, item)
			}

		}

		resIssues = append(resIssues, issue)
	}

	return
}

func ParsePlannedDate(ts string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", ts, time.UTC)
}

// jira format: 2019-07-12T22:32:50.376+0200
const jiraTimeFormat = "2006-01-02T15:04:05.999Z0700"

func ParseTime(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	return time.Parse(jiraTimeFormat, ts)
}
