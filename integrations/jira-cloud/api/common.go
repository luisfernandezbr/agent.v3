package api

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/jira/commonapi"
)

type QueryContext struct {
	WebsiteURL string
	Logger     hclog.Logger
	CustomerID string
	Req        commonapi.Requester
}

type PageInfo struct {
	Total      int
	MaxResults int
	HasMore    bool
}

func (s *QueryContext) Common() commonapi.QueryContext {
	res := commonapi.QueryContext{}
	res.WebsiteURL = s.WebsiteURL
	res.CustomerID = s.CustomerID
	res.Logger = s.Logger
	res.ExportUser = nil
	res.Req = s.Req
	res.Validate()
	return res
}
