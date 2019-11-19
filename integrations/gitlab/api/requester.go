package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/httpdefaults"
	pstrings "github.com/pinpt/go-common/strings"
)

type RequesterOpts struct {
	Logger             hclog.Logger
	APIURL             string
	APIToken           string
	InsecureSkipVerify bool
	ServerType         ServerType
}

type Requester struct {
	logger     hclog.Logger
	opts       RequesterOpts
	httpClient *http.Client
}

func NewRequester(opts RequesterOpts) *Requester {
	s := &Requester{}
	s.opts = opts
	s.logger = opts.Logger

	{
		c := &http.Client{}
		transport := httpdefaults.DefaultTransport()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify}
		c.Transport = transport
		s.httpClient = c
	}

	return s
}

func (s *Requester) setAuthHeader(req *http.Request) {
	if s.opts.ServerType == CLOUD {
		req.Header.Set("Authorization", "bearer "+s.opts.APIToken)
	} else {
		req.Header.Set("Private-Token", s.opts.APIToken)
	}
}

func (s *Requester) Request(objPath string, params url.Values, res interface{}) (page PageInfo, err error) {

	u := pstrings.JoinURL(s.opts.APIURL, objPath)

	if len(params) != 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return page, err
	}
	req.Header.Set("Accept", "application/json")
	s.setAuthHeader(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return page, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		s.logger.Debug("api request failed", "url", u)

		return page, fmt.Errorf(`gitlab returned invalid status code: %v`, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return page, err
	}

	rawPageSize := resp.Header.Get("X-Per-Page")

	var pageSize int
	if rawPageSize != "" {
		pageSize, err = strconv.Atoi(rawPageSize)
		if err != nil {
			return page, err
		}
	}

	rawTotalSize := resp.Header.Get("X-Total")

	var total int
	if rawTotalSize != "" {
		total, err = strconv.Atoi(rawTotalSize)
		if err != nil {
			return page, err
		}
	}

	return PageInfo{
		PageSize:   pageSize,
		NextPage:   resp.Header.Get("X-Next-Page"),
		Page:       resp.Header.Get("X-Page"),
		TotalPages: resp.Header.Get("X-Total-Pages"),
		Total:      total,
	}, nil
}
