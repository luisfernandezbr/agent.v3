package api

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
)

type QueryContext struct {
	Logger     hclog.Logger
	CustomerID string
	Request    func(objPath string, params url.Values, res interface{}) error
}

type PageInfo struct {
	Total      int
	MaxResults int
	HasMore    bool
}

func (s *QueryContext) common() jiracommonapi.QueryContext {
	res := jiracommonapi.QueryContext{}
	res.CustomerID = s.CustomerID
	res.Logger = s.Logger
	res.ExportUser = nil
	res.Request = s.Request
	return res
}
