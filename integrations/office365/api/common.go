package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/ids2"
	"github.com/pinpt/go-common/v10/httpdefaults"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/httpclient"
	"github.com/pinpt/integration-sdk/calendar"
)

type API interface {
	GetEventsAndUsers(calid string) ([]*calendar.Event, map[string]*calendar.User, error)
	GetMainCalendars() ([]*calendar.Calendar, error)
	GetSharedCalendars() ([]*calendar.Calendar, error)
	Validate() error
}

type api struct {
	logger       hclog.Logger
	refreshToken refreshTokenFunc
	client       *httpclient.HTTPClient
	customerID   string
	refType      string
	ids          ids2.Gen
	accessToken  string
}
type refreshTokenFunc = func() (string, error)

func New(logger hclog.Logger, customerID string, refType string, refreshToken refreshTokenFunc) (API, error) {
	client := &http.Client{
		Transport: httpdefaults.DefaultTransport(),
		Timeout:   10 * time.Minute,
	}
	conf := &httpclient.Config{
		Paginator: paginator{},
		Retryable: httpclient.NewBackoffRetry(10*time.Millisecond, 100*time.Millisecond, 60*time.Second, 2.0),
	}
	accessToken, err := refreshToken()
	if err != nil {
		return nil, err
	}
	return &api{
		client:      httpclient.NewHTTPClient(context.Background(), conf, client),
		logger:      logger,
		customerID:  customerID,
		refType:     refType,
		ids:         ids2.New(customerID, refType),
		accessToken: accessToken,
	}, nil
}

type queryParams map[string]string

func (s *api) get(u string, params queryParams, res interface{}) error {
	// ========== create request ==========
	requesturl, _ := url.Parse(pstrings.JoinURL("https://graph.microsoft.com/v1.0/", u))
	vals := requesturl.Query()
	for k, v := range params {
		vals.Set(k, v)
	}
	requesturl.RawQuery = vals.Encode()
	req, err := http.NewRequest(http.MethodGet, requesturl.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating request. err %v", err)
	}
	req.Header.Set("Prefer", `outlook.body-content-type="text"`)
	req.Header.Set("Authorization", "Bearer "+s.accessToken)
	// ========== do the request ==========
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error calling http client. err %v", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// ========== parse the response ==========
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response body. err %v", err)
		}
		// ========== for paging, join the lines ==========
		stringres := "[" + strings.Replace(string(b), "\n", ",", -1) + "]"
		err = json.Unmarshal([]byte(stringres), &res)
		if err != nil {
			return fmt.Errorf("error unmarshaling response. err %v res %v", err, stringres)
		}
	case http.StatusUnauthorized:
		if s.accessToken, err = s.refreshToken(); err != nil {
			return err
		}
		return s.get(u, params, req)
	default:
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response body. err %v", err)
		}
		return fmt.Errorf("error fetching from google calendar api. response_code: %v. response: %v", resp.StatusCode, string(b))
	}
	return nil
}
