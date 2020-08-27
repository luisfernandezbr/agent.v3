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
	GetEventsAndUsers(string) ([]*calendar.Event, map[string]*calendar.User, error)
	GetCalendar(calID string) (*calendar.Calendar, error)
	GetCalendars() ([]*calendar.Calendar, error)
	Validate() error
}

type refreshTokenFunc = func() (string, error)

type api struct {
	logger           hclog.Logger
	client           *httpclient.HTTPClient
	customerID       string
	refType          string
	ids              ids2.Gen
	refreshTokenFunc refreshTokenFunc
	accessToken      string
	lastTimeRetried  time.Time
}

// New creates a new instance
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
		client:           httpclient.NewHTTPClient(context.Background(), conf, client),
		logger:           logger,
		customerID:       customerID,
		refType:          refType,
		ids:              ids2.New(customerID, refType),
		accessToken:      accessToken,
		refreshTokenFunc: refreshToken,
	}, nil
}

type queryParams map[string]string

func (s *api) get(u string, params queryParams, res interface{}) error {
	// ========== create request ==========
	requesturl, _ := url.Parse(pstrings.JoinURL("https://www.googleapis.com/calendar/v3/", u))
	vals := requesturl.Query()
	for k, v := range params {
		vals.Set(k, v)
	}
	requesturl.RawQuery = vals.Encode()
	req, err := http.NewRequest(http.MethodGet, requesturl.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating request. err %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		return nil
	case http.StatusUnauthorized:
		if s.lastTimeRetried.IsZero() || time.Since(s.lastTimeRetried) > (5*time.Minute) {
			var err error
			s.lastTimeRetried = time.Now()
			s.accessToken, err = s.refreshTokenFunc()
			if err != nil {
				return err
			}
			return s.get(u, params, req)
		}
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body. err %v", err)
	}
	return fmt.Errorf("error fetching from google calendar api. response_code: %v. response: %v", resp.StatusCode, string(b))
}
