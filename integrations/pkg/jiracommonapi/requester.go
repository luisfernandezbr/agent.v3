package jiracommonapi

import (
	"net/url"

	"github.com/pinpt/agent/pkg/requests2"
)

type Requester interface {
	// Get and Get2 are for Get requests only
	Get(objPath string, params url.Values, res interface{}) error
	Get2(objPath string, params url.Values, res interface{}) (statusCode int, _ error)
	GetAgile(objPath string, params url.Values, res interface{}) error

	// JSON supports more configuration of request params
	JSON(req requests2.Request, res interface{}) (_ requests2.Result, rerr error)

	URL(objPath string) string
}
