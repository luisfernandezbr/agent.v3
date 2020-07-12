package api

import (
	"net/url"

	"github.com/pinpt/agent/pkg/ids2"

	"github.com/hashicorp/go-hclog"
)

type QueryContext struct {
	BaseURL string
	Logger  hclog.Logger
	Request func(string, url.Values, bool, interface{}, NextPage) (NextPage, error)

	CustomerID string
	RefType    string

	IDs ids2.Gen
}

type NextPage string
