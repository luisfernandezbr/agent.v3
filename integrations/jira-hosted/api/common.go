package api

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/hash"

	pstrings "github.com/pinpt/go-common/strings"
)

type QueryContext struct {
	BaseURL    string
	Logger     hclog.Logger
	CustomerID string
	Request    func(objPath string, params url.Values, res interface{}) error
	ExportUser func(user User) error
}

func (s QueryContext) IssueURL(issueKey string) string {
	return pstrings.JoinURL(s.BaseURL, "browse", issueKey)
}

func (s QueryContext) ProjectID(refID string) string {
	return hash.Values("Project", s.CustomerID, "work.Project", refID)
}

func (s QueryContext) IssueID(refID string) string {
	return hash.Values("Issue", s.CustomerID, "work.Issue", refID)
}

func (s QueryContext) UserID(refID string) string {
	return hash.Values("User", s.CustomerID, "work.User", refID)
}

type Project struct {
	JiraID string
	Key    string
}

type PageInfo struct {
	Total      int
	MaxResults int
	HasMore    bool
}
