package api

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
)

type QueryContext struct {
	BaseURL        string
	Logger         hclog.Logger
	Request        func(objPath string, params url.Values, res interface{}) (PageInfo, error)
	RequestGraphQL func(query string, res interface{}) (err error)

	CustomerID string
	RefType    string

	UserEmailMap map[string]string
}

type PageInfo struct {
	PageSize   int
	NextPage   string
	Page       string
	TotalPages string
}
