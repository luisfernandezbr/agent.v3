package api

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/jira/jiracommonapi"
)

type QueryContext struct {
	WebsiteURL string
	Logger     hclog.Logger
	CustomerID string
	Req        jiracommonapi.Requester
}

type PageInfo struct {
	Total      int
	MaxResults int
	HasMore    bool
}

func (s *QueryContext) Common() jiracommonapi.QueryContext {
	res := jiracommonapi.QueryContext{}
	res.WebsiteURL = s.WebsiteURL
	res.CustomerID = s.CustomerID
	res.Logger = s.Logger
	res.ExportUser = nil
	res.Req = s.Req
	res.Validate()
	return res
}
