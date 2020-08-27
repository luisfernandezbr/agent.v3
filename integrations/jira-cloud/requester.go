package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/pinpt/agent/pkg/oauthtoken"
	"github.com/pinpt/agent/pkg/reqstats"
	"github.com/pinpt/agent/pkg/requests"
	pstrings "github.com/pinpt/go-common/v10/strings"

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

func (s *Requester) Get(objPath string, params url.Values, res interface{}) error {
	_, err := s.get(objPath, params, res, 1, false)
	return err
}

func (s *Requester) Get2(objPath string, params url.Values, res interface{}) (statusCode int, _ error) {
	return s.get(objPath, params, res, 1, false)
}

func (s *Requester) GetAgile(objPath string, params url.Values, res interface{}) error {
	_, err := s.get(objPath, params, res, 1, true)
	return err
}

func (s *Requester) get(objPath string, params url.Values, res interface{}, maxOAuthRetries int, useAgile bool) (statusCode int, rerr error) {
	req := requests.NewRequest()

	var u string
	if useAgile {
		u = pstrings.JoinURL(s.opts.APIURL, "rest/agile/1.0", objPath)
	} else {
		u = pstrings.JoinURL(s.opts.APIURL, "rest/api", s.version, objPath)
	}

	if len(params) != 0 {
		u += "?" + params.Encode()
	}
	req.URL = u
	resp, err := s.json(req, res, maxOAuthRetries)
	if resp.Resp != nil {
		statusCode = resp.Resp.StatusCode
	}
	rerr = err
	return
}

func (s *Requester) URL(objPath string) string {
	return pstrings.JoinURL(s.opts.APIURL, "rest/api", s.version, objPath)
}

func (s *Requester) JSON(req requests.Request, res interface{}) (_ requests.Result, rerr error) {
	return s.json(req, res, 1)
}

func (s *Requester) json(req requests.Request, res interface{}, maxOAuthRetries int) (resp requests.Result, rerr error) {

	var reqs requests.Requests
	if s.opts.RetryRequests {
		reqs = requests.NewRetryableDefault(s.logger, s.opts.Clients.TLSInsecure)
	} else {
		reqs = requests.New(s.logger, s.opts.Clients.TLSInsecure)
	}

	if s.opts.OAuthToken != nil {
		if req.Header == nil {
			req.Header = http.Header{}
		}
		req.Header.Set("Authorization", "Bearer "+s.opts.OAuthToken.Get())
	} else {
		req.BasicAuthUser = s.opts.Username
		req.BasicAuthPassword = s.opts.Password
	}

	var err error
	resp, err = reqs.JSON(req, res)

	if s.opts.OAuthToken != nil {
		if resp.Resp != nil && resp.Resp.StatusCode == 401 {
			if maxOAuthRetries == 0 {
				rerr = fmt.Errorf("received error 401 after retrying with new oauth token, uri: %v", req.URL)
				return
			}
			err := s.opts.OAuthToken.Refresh()
			if err != nil {
				rerr = err
				return
			}
			return s.json(req, res, maxOAuthRetries-1)
		}
	}
	if err != nil {
		rerr = err
		return
	}

	return
}
