package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/hashicorp/go-hclog"
)

type RequesterOpts struct {
	Logger   hclog.Logger
	APIURL   string
	Username string
	Password string
}

type Requester struct {
	logger hclog.Logger
	opts   RequesterOpts
}

func NewRequester(opts RequesterOpts) *Requester {
	s := &Requester{}
	s.opts = opts
	s.logger = opts.Logger
	return s
}

const apiVersion = "3"

func (s *Requester) Request(objPath string, params url.Values, res interface{}) error {
	u := pstrings.JoinURL(s.opts.APIURL, "rest/api", apiVersion, objPath)

	if len(params) != 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest("GET", u, nil)
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
		s.logger.Info("api request failed", "body", string(b))
		return fmt.Errorf(`resp resp.StatusCode != 200, got %v`, resp.StatusCode)
	}

	s.logger.Info("res", "body", string(b))
	err = json.Unmarshal(b, &res)
	if err != nil {
		return err
	}

	return nil
}
