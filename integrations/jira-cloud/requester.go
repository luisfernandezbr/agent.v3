package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/pinpt/agent/pkg/oauthtoken"
	"github.com/pinpt/agent/pkg/reqstats"
	"github.com/pinpt/agent/pkg/requests"
	pstrings "github.com/pinpt/go-common/strings"

	"github.com/hashicorp/go-hclog"
)

type RequesterOpts struct {
	Logger        hclog.Logger
	Clients       reqstats.Clients
	APIURL        string
	Username      string
	Password      string
	OAuthToken    *oauthtoken.Manager
	RetryRequests bool
}

type Requester struct {
	logger hclog.Logger
	opts   RequesterOpts

	version string
}

func NewRequester(opts RequesterOpts) *Requester {
	s := &Requester{}
	s.opts = opts
	s.logger = opts.Logger

	s.version = "3"
	return s
}

func (s *Requester) Request(objPath string, params url.Values, res interface{}) error {
	_, err := s.request(objPath, params, res, 1)
	return err
}

func (s *Requester) Request2(objPath string, params url.Values, res interface{}) (statusCode int, _ error) {
	return s.request(objPath, params, res, 1)
}

func (s *Requester) request(objPath string, params url.Values, res interface{}, maxOAuthRetries int) (statusCode int, rerr error) {
	u := pstrings.JoinURL(s.opts.APIURL, "rest/api", s.version, objPath)
	if len(params) != 0 {
		u += "?" + params.Encode()
	}
	var reqs requests.Requests
	if s.opts.RetryRequests {
		reqs = requests.NewRetryableDefault(s.logger, s.opts.Clients.TLSInsecure)
	} else {
		reqs = requests.New(s.logger, s.opts.Clients.TLSInsecure)
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		rerr = err
		return
	}

	if s.opts.OAuthToken != nil {
		req.Header.Set("Authorization", "Bearer "+s.opts.OAuthToken.Get())
	} else {
		req.SetBasicAuth(s.opts.Username, s.opts.Password)
	}

	resp, err := reqs.JSON(req, res)
	if resp != nil {
		statusCode = resp.StatusCode
	}
	if s.opts.OAuthToken != nil {
		if resp != nil && resp.StatusCode == 401 {
			if maxOAuthRetries == 0 {
				rerr = fmt.Errorf("received error 401 after retrying with new oauth token, path: %v", objPath)
				return
			}
			err := s.opts.OAuthToken.Refresh()
			if err != nil {
				rerr = err
				return
			}
			return s.request(objPath, params, res, maxOAuthRetries-1)
		}
	}
	if err != nil {
		rerr = err
		return
	}

	return
}
