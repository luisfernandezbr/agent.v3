package api

import (
	"net/url"

	"github.com/pinpt/agent/pkg/ids2"

	"github.com/hashicorp/go-hclog"
)

type QueryContext struct {
	BaseURL string
	Logger  hclog.Logger
	Request func(string, url.Values, bool, interface{}) (PageInfo, error)

	CustomerID string
	RefType    string

	IDs ids2.Gen
}

type PageInfo struct {
	PageSize int64
	NextPage string
	Page     int64
	Total    int
}
