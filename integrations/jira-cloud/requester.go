package main

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/oauthtoken"
	"github.com/pinpt/agent.next/pkg/reqstats"
	"github.com/pinpt/agent.next/pkg/requests"
	pstrings "github.com/pinpt/go-common/strings"

	"github.com/hashicorp/go-hclog"
)

type RequesterOpts struct {
	Logger     hclog.Logger
	Clients    reqstats.Clients
	APIURL     string
	Username   string
	Password   string
	OAuthToken *oauthtoken.Manager
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
	return s.request(objPath, params, res, 1)
}

func (s *Requester) request(objPath string, params url.Values, res interface{}, maxOAuthRetries int) error {
	u := pstrings.JoinURL(s.opts.APIURL, "rest/api", s.version, objPath)

	if len(params) != 0 {
		u += "?" + params.Encode()
	}

	// retry 10 times, 500 millis per retry, and max timeout 1 minute
	reqs := requests.NewRetryable(s.logger, s.opts.Clients.Default, 10, 500*time.Millisecond, 1*time.Minute)

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}

	if s.opts.OAuthToken != nil {
		req.Header.Set("Authorization", "Bearer "+s.opts.OAuthToken.Get())
	} else {
		req.SetBasicAuth(s.opts.Username, s.opts.Password)
	}

	resp, err := reqs.JSON(req, res)
	if s.opts.OAuthToken != nil {
		if resp.StatusCode == 401 {
			if maxOAuthRetries == 0 {
				return fmt.Errorf("received error 401 after retrying with new oauth token, path: %v", objPath)
			}
			err := s.opts.OAuthToken.Refresh()
			if err != nil {
				return err
			}
			return s.request(objPath, params, res, maxOAuthRetries-1)
		}
	}
	if err != nil {
		return err
	}

	return nil
}
