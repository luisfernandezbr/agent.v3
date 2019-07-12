package api

import (
	"fmt"
	"net/url"
	"time"

	"github.com/pinpt/go-datamodel/work"
)

func IssuesPage(
	qc QueryContext,
	project Project,
	paginationParams url.Values) (pi PageInfo, res []*work.Issue, _ error) {

	objectPath := "search"
	params := paginationParams

	//params.Set("maxResults", "1") // for testing
	params.Set("validateQuery", "strict")
	params.Set("jql", "project="+project.JiraID)

	qc.Logger.Debug("issues request", "project", project.Key, "params", params)

	var rr struct {
		Total      int `json:"total"`
		MaxResults int `json:"maxResults"`
		Issues     []struct {
			ID     string `json:"id"`
			Key    string `json:"key"`
			Fields struct {
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
				Creator struct {
					AccountID string `json:"accountId"`
				} `json:"creator"`
				Reporter struct {
					AccountID string `json:"accountId"`
				} `json:"reporter"`
				Assignee struct {
					AccountID string `json:"accountId"`
				} `json:"assignee"`
			} `json:"fields"`
		} `json:"issues"`
	}

	err := qc.Request(objectPath, params, &rr)
	if err != nil {
		return pi, res, err
	}

	pi.Total = rr.Total
	pi.MaxResults = rr.MaxResults
	if len(rr.Issues) == rr.MaxResults {
		pi.HasMore = true
	}

	for _, data := range rr.Issues {
		item := &work.Issue{}
		item.CustomerID = qc.CustomerID
		item.RefID = data.ID
		item.RefType = "jira"
		item.Identifier = data.Key
		item.ProjectID = qc.ProjectID(project.JiraID)

		fields := data.Fields
		if fields.DueDate != "" {
			orig := fields.DueDate
			d, err := time.ParseInLocation("2006-01-02", orig, time.UTC)
			if err != nil {
				return pi, res, fmt.Errorf("could not parse duedate of jira issue: %v err: %v", orig, err)
			}
			item.DueDateAt = d.Unix()
		}

		item.Title = fields.Summary

		item.CreatedAt, err = parseTime(fields.Created)
		if err != nil {
			return pi, res, err
		}
		item.UpdatedAt, err = parseTime(fields.Updated)
		if err != nil {
			return pi, res, err
		}

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
		}
		if fields.Reporter.AccountID != "" {
			item.ReporterRefID = fields.Reporter.AccountID
		}
		if fields.Assignee.AccountID != "" {
			item.AssigneeRefID = fields.Assignee.AccountID
		}

		// TODO:
		//   - url
		//   - tags
		//   - customFields
		//   - parent_id

		res = append(res, item)
	}

	return pi, res, nil
}

// jira format: 2019-07-12T22:32:50.376+0200
const jiraTimeFormat = "2006-01-02T15:04:05.999Z0700"

func parseTime(ts string) (int64, error) {
	if ts == "" {
		return 0, nil
	}
	res, err := time.Parse(jiraTimeFormat, ts)
	if err != nil {
		return 0, err
	}
	return res.Unix(), nil
}
