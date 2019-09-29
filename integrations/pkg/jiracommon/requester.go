package jiracommon

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pinpt/go-common/httpdefaults"
	pstrings "github.com/pinpt/go-common/strings"

	"github.com/hashicorp/go-hclog"
)

type RequesterOpts struct {
	Logger   hclog.Logger
	APIURL   string
	Username string
	Password string

	IsJiraCloud bool
}

type Requester struct {
	logger     hclog.Logger
	opts       RequesterOpts
	httpClient *http.Client

	version string
}

func NewRequester(opts RequesterOpts) *Requester {
	s := &Requester{}
	s.opts = opts
	s.logger = opts.Logger

	if s.opts.IsJiraCloud {
		s.httpClient = http.DefaultClient
		s.version = "3"
	} else {
		c := &http.Client{}
		transport := httpdefaults.DefaultTransport()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		c.Transport = transport
		s.httpClient = c
		s.version = "2"
	}

	return s
}

func NewRequesterFromConfig(logger hclog.Logger, conf Config, isJiraCloud bool) *Requester {
	opts := RequesterOpts{}
	opts.Logger = logger
	opts.APIURL = conf.URL
	opts.Username = conf.Username
	opts.Password = conf.Password
	opts.IsJiraCloud = isJiraCloud
	return NewRequester(opts)
}

func (s *Requester) Request(objPath string, params url.Values, res interface{}) error {
	u := pstrings.JoinURL(s.opts.APIURL, "rest/api", s.version, objPath)

	if len(params) != 0 {
		u += "?" + params.Encode()
	}
	s.logger.Info("request", "url", u)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.opts.Username, s.opts.Password)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		//s.logger.Info("api request failed", "body", string(b))
		return fmt.Errorf(`jira returned invalid status code: %v`, resp.StatusCode)
	}

	//s.logger.Info("res", "body", string(b))
	err = json.Unmarshal(b, &res)
	if err != nil {
		return err
	}

	return nil
}
