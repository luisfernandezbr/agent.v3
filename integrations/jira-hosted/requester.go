package main

import (
	"net/http"
	"net/url"

	"github.com/pinpt/agent.next/pkg/reqstats"
	"github.com/pinpt/agent.next/pkg/requests"
	pstrings "github.com/pinpt/go-common/strings"

	"github.com/hashicorp/go-hclog"
)

type RequesterOpts struct {
	Logger   hclog.Logger
	Clients  reqstats.Clients
	APIURL   string
	Username string
	Password string
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

	s.version = "2"
	return s
}

func (s *Requester) Request(objPath string, params url.Values, res interface{}) error {
	u := pstrings.JoinURL(s.opts.APIURL, "rest/api", s.version, objPath)

	if len(params) != 0 {
		u += "?" + params.Encode()
	}

	reqs := requests.New(s.logger, s.opts.Clients.TLSInsecure)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(s.opts.Username, s.opts.Password)

	_, err = reqs.JSON(req, res)
	if err != nil {
		return err
	}

	return nil
}
