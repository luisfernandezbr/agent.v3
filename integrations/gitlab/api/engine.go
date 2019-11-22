package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/hashicorp/go-hclog"

	pstrings "github.com/pinpt/go-common/strings"
)

// Do not have more than 10 api calls in progress
const limit = 5

type internalRequest struct {
	URL      string
	Params   url.Values
	Done     chan bool
	Response interface{}
	PageInfo PageInfo
	Error    error
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

// Engine str
type Engine struct {
	channel  chan *internalRequest
	client   *http.Client
	err      *errorState
	ctx      context.Context
	cancel   context.CancelFunc
	apiURL   string
	apiToken string
	logger   hclog.Logger
}

// SetClient set client
func (e *Engine) SetClient(c *http.Client) {
	e.client = c
}

// SetupEngine setup and start engine
func (e *Engine) SetupEngine(apiUrl string, apiToken string, logger hclog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.cancel = cancel
	e.channel = make(chan *internalRequest)
	e.err = &errorState{}
	e.apiURL = apiUrl
	e.apiToken = apiToken
	e.logger = logger
}

// StartEngine setup and start engine
func (e *Engine) StartEngine() {
	go e.engine()
}

func (e *Engine) addRequest(r *internalRequest) {
	e.channel <- r
}

func (e *Engine) Error() error {
	return e.err.getError()
}

func (e *Engine) purge() {
	for p := range e.channel {
		p.Done <- true
	}
}

func (e *Engine) engine() {

	sem := make(chan bool, limit)

	var locker sync.Mutex
	var errorOccurr bool
	setError := func(err error) {
		locker.Lock()
		defer locker.Unlock()
		if !errorOccurr {
			// fmt.Println("Error", err)
			e.cancel()
			e.err.setError(err)
			errorOccurr = true
		}
	}
	isError := func() bool {
		locker.Lock()
		defer locker.Unlock()
		return errorOccurr
	}

	for req := range e.channel {
		sem <- true
		go func(req *internalRequest) {
			defer func() { <-sem }()
			e.logger.Info("About to run ", "url", req.URL)
			if err := e.request(req); err != nil {
				setError(err)
				return
			}
		}(req)
		if isError() {
			break
		}
	}

	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	if err := e.err.getError(); err != nil {
		go e.purge()
		fmt.Println("Mi error", err)
	}

}

// MakeRequest mr
func (e *Engine) MakeRequest(url string, params url.Values, response interface{}) (PageInfo, error) {

	ir := internalRequest{
		URL:      url,
		Done:     make(chan bool, 1),
		Response: &response,
		Params:   params,
	}

	e.addRequest(&ir)

	<-ir.Done

	return ir.PageInfo, ir.Error

}

func (e *Engine) setAuthHeader(req *http.Request) {
	// if s.ServerType == CLOUD {
	// 	req.Header.Set("Authorization", "bearer "+e.apiToken)
	// } else {
	req.Header.Set("Private-Token", e.apiToken)
	// }
}

func (e *Engine) request(r *internalRequest) error {

	defer func() { r.Done <- true }()

	u := pstrings.JoinURL(e.apiURL, r.URL)

	if len(r.Params) != 0 {
		u += "?" + r.Params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		r.Error = err
		return err
	}
	req.Header.Set("Accept", "application/json")
	e.setAuthHeader(req)

	req = req.WithContext(e.ctx)

	resp, err := e.client.Do(req)
	if err != nil {
		r.Error = err
		return err
	}
	defer resp.Body.Close()

	// for k, v := range resp.Header {
	// 	if strings.Contains("RateLimit", k) {
	// 		s.logger.Debug(">>>>>>>>>>>", k, strings.Join(v, ","))
	// 	}
	// }

	if resp.StatusCode != 200 {
		// s.logger.Debug("api request failed", "url", u)

		// bts, _ := ioutil.ReadAll(resp.Body)
		// s.logger.Debug("response", "BODY", string(bts))

		err := fmt.Errorf(`gitlab returned invalid status code: %v`, resp.StatusCode)
		r.Error = err
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(&r.Response); err != nil {
		r.Error = err
		return err
	}

	rawPageSize := resp.Header.Get("X-Per-Page")

	var pageSize int
	if rawPageSize != "" {
		pageSize, err = strconv.Atoi(rawPageSize)
		if err != nil {
			r.Error = err
			return err
		}
	}

	rawTotalSize := resp.Header.Get("X-Total")

	var total int
	if rawTotalSize != "" {
		total, err = strconv.Atoi(rawTotalSize)
		if err != nil {
			r.Error = err
			return err
		}
	}

	r.PageInfo = PageInfo{
		PageSize:   pageSize,
		NextPage:   resp.Header.Get("X-Next-Page"),
		Page:       resp.Header.Get("X-Page"),
		TotalPages: resp.Header.Get("X-Total-Pages"),
		Total:      total,
	}

	return nil
}

// // PageInfo page info
// type PageInfo struct {
// 	PageSize   int
// 	NextPage   string
// 	Page       string
// 	TotalPages string
// 	Total      int
// }
