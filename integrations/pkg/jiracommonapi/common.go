package jiracommonapi

import (
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent/pkg/ids"
	pstrings "github.com/pinpt/go-common/strings"
)

type QueryContext struct {
	WebsiteURL string
	Logger     hclog.Logger
	CustomerID string
	ExportUser func(user User) error
	Req        Requester
}

func (s QueryContext) Validate() {
	if s.WebsiteURL == "" || s.Logger == nil || s.CustomerID == "" || s.Req == nil {
		panic("set all required fields")
	}
}

func (s QueryContext) ProjectURL(projectKey string) string {
	return pstrings.JoinURL(s.WebsiteURL, "browse", projectKey)
}

func (s QueryContext) IssueURL(issueKey string) string {
	return pstrings.JoinURL(s.WebsiteURL, "browse", issueKey)
}

func (s QueryContext) IssueCommentURL(issueKey string, commentID string) string {
	// looks like: https://pinpt-hq.atlassian.net/browse/DE-2194?focusedCommentId=17692&page=com.atlassian.jira.plugin.system.issuetabpanels%3Acomment-tabpanel#comment-17692
	return pstrings.JoinURL(s.WebsiteURL, "browse", issueKey+fmt.Sprintf("?focusedCommentId=%s&page=com.atlassian.jira.plugin.system.issuetabpanels%%3Acomment-tabpanel#comment-%s", commentID, commentID))
}

func (s QueryContext) ProjectID(refID string) string {
	return ids.WorkProject(s.CustomerID, "jira", refID)
}
func (s QueryContext) SprintID(refID string) string {
	return ids.WorkSprint(s.CustomerID, "jira", refID)
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
