package jiracommonapi

import (
	"errors"
	"net/url"
	"time"
)

type ProjectLastIssue struct {
	IssueID     string
	Identifier  string
	CreatedDate time.Time
	Creator     User
}

var ErrPermissions = errors.New("insufficient permissions")

func GetProjectLastIssue(qc QueryContext, project Project) (res ProjectLastIssue, totalIssues int, rerr error) {

	q := url.Values{}
	q.Set("jql", `project="`+project.Key+`"`)
	q.Set("maxResults", "1")
	//q.Set("orderBy", "-created") This does not work, but issues are sorted by deafault newest first
	q.Set("fields", "created,creator")

	objectPath := "search"
	qc.Logger.Debug("project last issue", "project", project.JiraID)

	var rr struct {
		Total  int `json:"total"`
		Issues []struct {
			ID     string `json:"id"`
			Key    string `json:"key"`
			Fields struct {
				Creator User   `json:"creator"`
				Created string `json:"created"`
			} `json:"fields"`
		} `json:"issues"`
	}

	statusCode, err := qc.Req.Get2(objectPath, q, &rr)
	if err != nil {
		if statusCode == 400 {
			qc.Logger.Error("failed getting last project issue, probably due to insufficient permissions", "err", err)
			rerr = ErrPermissions
			return
		}
		rerr = err
		return
	}

	if len(rr.Issues) == 0 {
		return
	}

	data := rr.Issues[0]
	res.IssueID = data.ID
	res.Identifier = data.Key
	res.CreatedDate, err = ParseTime(data.Fields.Created)
	if err != nil {
		rerr = err
		return
	}
	res.Creator = data.Fields.Creator
	totalIssues = rr.Total

	return
}
