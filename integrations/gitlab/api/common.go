package api

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
)

type QueryContext struct {
	BaseURL string
	Logger  hclog.Logger
	Request func(objPath string, params url.Values, res interface{}) error

	APIURL3 string

	CustomerID string
	RefType    string

	UserLoginToRefID           func(login string) (refID string, _ error)
	UserLoginToRefIDFromCommit func(login, email string) (refID string, _ error)

	IsEnterprise func() bool
}
