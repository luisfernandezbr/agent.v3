package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/structmarshal"

	"github.com/pinpt/go-datamodel/work"
)

func IssuesAndChangelogsPage(
	qc QueryContext,
	project Project,
	fieldByKey map[string]*work.CustomField,
	updatedSince time.Time,
	paginationParams url.Values) (
	pi PageInfo,
	resIssues []*work.Issue,
	resChangelogs []*work.Changelog,

	rerr error) {

	objectPath := "search"
	params := paginationParams

	//params.Set("maxResults", "1") // for testing
	params.Set("validateQuery", "strict")
	jql := "project=" + project.JiraID
	if !updatedSince.IsZero() {
		s := updatedSince.Format("2006-01-02 15:04")
		jql += fmt.Sprintf(` and (created >= "%s" or updated >= "%s")`, s, s)
	}
	params.Set("jql", jql)
	params.Add("expand", "changelog")

	qc.Logger.Debug("issues request", "project", project.Key, "params", params)

	var rr struct {
		Total      int `json:"total"`
		MaxResults int `json:"maxResults"`
		Issues     []struct {
			ID        string                 `json:"id"`
			Key       string                 `json:"key"`
			Fields    map[string]interface{} `json:"fields"`
			Changelog struct {
				Histories []struct {
					ID     string `json:"id"`
					Author struct {
						AccountID string `json:"accountId"`
					} `json:"author"`
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

	for i, data := range rr.Issues {

		fields := issuesTypedFields[i]

		item := &work.Issue{}
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

		created, err := ParseTime(fields.Created)
		if err != nil {
			rerr = err
			return
		}
		date.ConvertToModel(created, &item.Created)
		updated, err := ParseTime(fields.Updated)
		if err != nil {
			rerr = err
			return
		}
		date.ConvertToModel(updated, &item.Updated)

		// TODO: check if name or id should be here
		item.Priority = fields.Priority.Name
		// TODO: check if name or id should be here
		item.Type = fields.IssueType.Name
		// TODO: check if name or id should be here
		item.Status = fields.Status.Name
		// TODO: check if name or id should be here
		item.Resolution = fields.Resolution.Name

		// TODO: for account references do we want ids or emails?
		if fields.Creator.AccountID != "" {
			item.CreatorRefID = fields.Creator.AccountID
			err := qc.ExportUser(fields.Creator)
			if err != nil {
				rerr = err
				return
			}
		}
		if fields.Reporter.AccountID != "" {
			item.ReporterRefID = fields.Reporter.AccountID
			err := qc.ExportUser(fields.Reporter)
			if err != nil {
				rerr = err
				return
			}
		}
		if fields.Assignee.AccountID != "" {
			item.AssigneeRefID = fields.Assignee.AccountID
			err := qc.ExportUser(fields.Assignee)
			if err != nil {
				rerr = err
				return
			}
		}

		item.URL = qc.IssueURL(data.Key)
		item.Tags = fields.Labels

		// TODO: this is different from previous agent and is currently handled in pipeline repo. pkg/legacy/jira
		for k, d := range data.Fields {
			if !strings.HasPrefix(k, "customfield_") {
				continue
			}

			fd, ok := fieldByKey[k]
			if !ok {
				rerr = fmt.Errorf("could not find field defintion using key: %v", k)
				return
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

			f := work.IssueCustomFields{}
			f.ID = fd.Key
			f.Name = fd.Name
			f.Value = v
			item.CustomFields = append(item.CustomFields, f)
		}

		// TODO: - parent_id
		// parent_id is used in pipeline, but not prev. agent. not sure when it's set in jira (not subtasks)

		issueRefID := item.RefID
		issueID := qc.IssueID(item.RefID)

		for i, cl := range data.Changelog.Histories {
			for _, data := range cl.Items {

				item := &work.Changelog{}
				item.CustomerID = qc.CustomerID
				item.RefType = "jira"
				item.RefID = cl.ID

				item.IssueID = issueID
				item.Ordinal = int64(i)

				createdAt, err := ParseTime(cl.Created)
				if err != nil {
					rerr = fmt.Errorf("could not parse created time of changelog for issue: %v err: %v", issueRefID, err)
					return
				}
				date.ConvertToModel(createdAt, &item.Created)
				item.UserID = qc.UserID(cl.Author.AccountID)

				item.Field = data.Field
				item.FieldType = data.FieldType
				item.From = data.From
				item.FromString = data.FromString
				item.To = data.To
				item.ToString = data.ToString
				resChangelogs = append(resChangelogs, item)
			}

		}

		resIssues = append(resIssues, item)
	}

	return
}

// jira format: 2019-07-12T22:32:50.376+0200
const jiraTimeFormat = "2006-01-02T15:04:05.999Z0700"

func ParseTime(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	return time.Parse(jiraTimeFormat, ts)
}

func ParseTimeUnix(ts string) (int64, error) {
	if ts == "" {
		return 0, nil
	}
	res, err := time.Parse(jiraTimeFormat, ts)
	if err != nil {
		return 0, err
	}
	return res.Unix(), nil
}
