package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/httpdefaults"
	pstrings "github.com/pinpt/go-common/strings"
)

type RequesterOpts struct {
	Logger   hclog.Logger
	APIURL   string
	APIToken string
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
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		c.Transport = transport
		s.httpClient = c
	}

	return s
}

func (s *Requester) Request(objPath string, params url.Values, res interface{}) error {

	u := pstrings.JoinURL(s.opts.APIURL, objPath)

	if len(params) != 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Private-Token", s.opts.APIToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		s.logger.Info("api request failed", "url", u, "body", string(b))
		return fmt.Errorf(`gitlab returned invalid status code: %v`, resp.StatusCode)
	}

	//s.logger.Info("res", "body", string(b))
	err = json.Unmarshal(b, &res)
	if err != nil {
		return err
	}

	return nil
}
