package api

import (
	"github.com/pinpt/agent/pkg/requests2"
	pstrings "github.com/pinpt/go-common/strings"
)

func newRestRequest(qc QueryContext, urlPath string) requests2.Request {
	req := requests2.NewRequest()
	req.URL = pstrings.JoinURL(qc.APIURL3, urlPath)
	req.Header.Set("Authorization", "token "+qc.AuthToken)
	return req
}
