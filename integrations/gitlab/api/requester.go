package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-common/v10/httpdefaults"
	pstrings "github.com/pinpt/go-common/v10/strings"
)

// RequesterOpts requester opts
type RequesterOpts struct {
	Logger             hclog.Logger
	APIURL             string
	APIKey             string
	AccessToken        string
	InsecureSkipVerify bool
	ServerType         ServerType
	Concurrency        chan bool
	Client             *http.Client
	Agent              rpcdef.Agent
}

// NewRequester new requester
func NewRequester(opts RequesterOpts) *Requester {
	re := &Requester{}
	{
		c := &http.Client{}
		transport := httpdefaults.DefaultTransport()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify}
		c.Transport = transport
		opts.Client = c
	}

	re.opts = opts

	return re
}

type internalRequest struct {
	URL      string
	Params   url.Values
	Response interface{}
	PageInfo PageInfo
}

type errorState struct {
	sync.Mutex
	err error
}

func (e *errorState) setError(err error) {
	e.Lock()
	defer e.Unlock()
	e.err = err
}

func (e *errorState) getError() error {
	e.Lock()
	defer e.Unlock()
	return e.err
}

// Requester requester
type Requester struct {
	opts RequesterOpts
}

// MakeRequest make request
func (e *Requester) MakeRequest(url string, params url.Values, response interface{}) (pi PageInfo, err error) {
	e.opts.Concurrency <- true
	defer func() {
		<-e.opts.Concurrency
	}()

	ir := internalRequest{
		URL:      url,
		Response: &response,
		Params:   params,
	}

	return e.makeRequestRetry(&ir, 0)

}

const maxGeneralRetries = 2

func (e *Requester) makeRequestRetry(req *internalRequest, generalRetry int) (pageInfo PageInfo, err error) {
	var isRetryable bool
	isRetryable, pageInfo, err = e.request(req, generalRetry+1)
	if err != nil {
		if !isRetryable {
			return pageInfo, err
		}
		if generalRetry >= maxGeneralRetries {
			return pageInfo, fmt.Errorf(`can't retry request, too many retries, err: %v`, err)
		}
		return e.makeRequestRetry(req, generalRetry+1)
	}
	return
}

func (e *Requester) setAuthHeader(req *http.Request) {
	if e.opts.APIKey == "" {
		req.Header.Set("Authorization", "bearer "+e.opts.AccessToken)
	} else {
		req.Header.Set("Private-Token", e.opts.APIKey)
	}
}

const maxThrottledRetries = 3

type errorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (e *Requester) request(r *internalRequest, retryThrottled int) (isErrorRetryable bool, pi PageInfo, rerr error) {
	u := pstrings.JoinURL(e.opts.APIURL, r.URL)

	if len(r.Params) != 0 {
		u += "?" + r.Params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return false, pi, err
	}
	req.Header.Set("Accept", "application/json")
	e.setAuthHeader(req)

	resp, err := e.opts.Client.Do(req)
	if err != nil {
		return true, pi, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		rerr = err
		isErrorRetryable = true
		return
	}
	rateLimited := func() (isErrorRetryable bool, pageInfo PageInfo, rerr error) {

		waitTime := time.Minute * 3

		e.opts.Logger.Warn("api request failed due to throttling, the quota of 600 calls has been reached, will sleep for 3m and retry", "retryThrottled", retryThrottled)

		paused := time.Now()
		resumeDate := paused.Add(waitTime)
		e.opts.Agent.SendPauseEvent(fmt.Sprintf("gitlab paused, it will resume in %v", waitTime), resumeDate)

		time.Sleep(waitTime)

		e.opts.Agent.SendResumeEvent(fmt.Sprintf("gitlab resumed, time elapsed %v", time.Since(paused)))

		return true, PageInfo{}, fmt.Errorf("Too many requests")

	}

	if resp.StatusCode != http.StatusOK {

		if resp.StatusCode == http.StatusTooManyRequests {
			return rateLimited()
		}

		if resp.StatusCode == http.StatusForbidden {

			var errorR *errorResponse

			er := json.Unmarshal([]byte(b), &errorR)
			if er != nil {
				return false, pi, fmt.Errorf("unmarshal error %s", er)
			}

			return false, pi, fmt.Errorf("%s, %s, scopes required: api, read_user, read_repository", errorR.Error, errorR.ErrorDescription)
		}

		e.opts.Logger.Warn("gitlab returned invalid status code, retrying", "code", resp.StatusCode, "retry", retryThrottled)

		return true, pi, fmt.Errorf("request with status %d", resp.StatusCode)
	}
	err = json.Unmarshal(b, &r.Response)
	if err != nil {
		rerr = err
		return
	}

	rawPageSize := resp.Header.Get("X-Per-Page")

	var pageSize int
	if rawPageSize != "" {
		pageSize, err = strconv.Atoi(rawPageSize)
		if err != nil {
			return false, pi, err
		}
	}

	rawTotalSize := resp.Header.Get("X-Total")

	var total int
	if rawTotalSize != "" {
		total, err = strconv.Atoi(rawTotalSize)
		if err != nil {
			return false, pi, err
		}
	}

	return false, PageInfo{
		PageSize:   pageSize,
		NextPage:   resp.Header.Get("X-Next-Page"),
		Page:       resp.Header.Get("X-Page"),
		TotalPages: resp.Header.Get("X-Total-Pages"),
		Total:      total,
	}, nil
}
