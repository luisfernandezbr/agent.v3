package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/httpdefaults"
	pstrings "github.com/pinpt/go-common/strings"
)

type RequesterOpts struct {
	Logger             hclog.Logger
	APIURL             string
	Username           string
	Password           string
	InsecureSkipVerify bool
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

type Requester struct {
	logger     hclog.Logger
	opts       RequesterOpts
	httpClient *http.Client
}

func (s *Requester) Request(objPath string, params url.Values, paginable bool, res interface{}) (page PageInfo, err error) {

	u := pstrings.JoinURL(s.opts.APIURL, objPath)

	if len(params) != 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return
	}
	req.SetBasicAuth(s.opts.Username, s.opts.Password)

	s.logger.Debug("request", "url", req.URL.String())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		s.logger.Info("api request failed", "url", u)
		return page, fmt.Errorf(`bitbucket returned invalid status code: %v`, resp.StatusCode)
	}

	if paginable {
		var response Response

		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return
		}

		if err = json.Unmarshal(response.Values, &res); err != nil {
			return
		}

		u, _ := url.Parse(response.Next)

		page.PageSize = response.PageLen
		page.Page = response.Page
		page.NextPage = u.Query().Get("page")

	} else {
		if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return page, err
		}
	}

	return
}

type Response struct {
	TotalValues int64           `json:"size"`
	Page        int64           `json:"page"`
	PageLen     int64           `json:"pagelen"`
	Next        string          `json:"next"`
	Values      json.RawMessage `json:"values"`
}
