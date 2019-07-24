package api

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
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
