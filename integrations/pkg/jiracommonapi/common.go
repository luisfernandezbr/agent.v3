package jiracommonapi

import (
	"net/url"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent.next/pkg/ids"
	pstrings "github.com/pinpt/go-common/strings"
)

type QueryContext struct {
	WebsiteURL string
	Logger     hclog.Logger
	CustomerID string
	Request    func(objPath string, params url.Values, res interface{}) error
	Request2   func(objPath string, params url.Values, res interface{}) (statusCode int, _ error)
	ExportUser func(user User) error
}

func (s QueryContext) Validate() {
	if s.WebsiteURL == "" || s.Logger == nil || s.CustomerID == "" || s.Request == nil {
		panic("set all required fields")
	}
}

func (s QueryContext) ProjectURL(projectKey string) string {
	return pstrings.JoinURL(s.WebsiteURL, "browse", projectKey)
}

func (s QueryContext) IssueURL(issueKey string) string {
	return pstrings.JoinURL(s.WebsiteURL, "browse", issueKey)
}

func (s QueryContext) ProjectID(refID string) string {
	return ids.WorkProject(s.CustomerID, "jira", refID)
}

func (s QueryContext) IssueID(refID string) string {
	return ids.WorkIssue(s.CustomerID, "jira", refID)
}

func (s QueryContext) UserID(refID string) string {
	return ids.WorkUser(s.CustomerID, "jira", refID)
}

type PageInfo struct {
	Total      int
	MaxResults int
	HasMore    bool
}

type Project struct {
	JiraID string
	Key    string
}
