package api

import (
	"net/url"

	"github.com/pinpt/agent.next/pkg/ids"

	"github.com/hashicorp/go-hclog"
)

type QueryContext struct {
	BaseURL string
	Logger  hclog.Logger
	Request func(string, url.Values, bool, interface{}) (PageInfo, error)

	CustomerID string
	RefType    string

	UserEmailMap map[string]string
	BasicInfo    ids.BasicInfo
}

type PageInfo struct {
	PageSize int64
	NextPage string
	Page     int64
}
