package api

import (
	"github.com/pinpt/agent/pkg/requests"
	pstrings "github.com/pinpt/go-common/v10/strings"
)

func newRestRequest(qc QueryContext, urlPath string) requests.Request {
	req := requests.NewRequest()
	req.URL = pstrings.JoinURL(qc.APIURL3, urlPath)
	req.Header.Set("Authorization", "token "+qc.AuthToken)
	return req
}
