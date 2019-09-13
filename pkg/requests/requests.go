package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/go-hclog"
)

type Requests struct {
	Logger hclog.Logger
	Client *http.Client
}

func New(logger hclog.Logger, client *http.Client) Requests {
	return Requests{
		Logger: logger,
		Client: client,
	}
}

// Do makes an http request. It preserves both request and response body for logging purposes.
// Returns logError function that logs the passed error together with request and response body for easier debugging.
func (opts Requests) Do(ctx context.Context, reqDef *http.Request) (resp *http.Response, logErrorWithRequest func(error) error, rerr error) {
	logger := opts.Logger
	u := reqDef.URL.String()

	req, reqBody, err := requestExtractBody(reqDef)
	if err != nil {
		rerr = err
		return
	}
	req.Header.Set("Accept", "application/json")
	req = req.WithContext(ctx)
	resp, err = opts.Client.Do(req)
	if err != nil {
		rerr = err
		return
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		rerr = err
		return
	}
	err = resp.Body.Close()
	if err != nil {
		rerr = err
		return
	}
	resp.Body = ioutil.NopCloser(bytes.NewReader(respBody))
	logErrorWithRequest = func(err error) error {
		logger.Debug("error processing response", "err", err.Error(), "url", u, "response_code", resp.StatusCode, "request_body", string(reqBody), "response_body", string(respBody))
		return fmt.Errorf("request failed url: %v err: %v", u, err)
	}
	return
}

func requestExtractBody(req *http.Request) (res *http.Request, reqBody []byte, rerr error) {
	var b []byte

	if req.Body != nil {
		var err error
		b, err = ioutil.ReadAll(req.Body)
		if err != nil {
			rerr = err
			return
		}
	}

	res, err := http.NewRequest(req.Method, req.URL.String(), bytes.NewReader(b))
	if err != nil {
		rerr = err
		return
	}
	res.Header = req.Header

	return res, b, nil
}

// JSON makes http request and unmarshals resulting json. Returns errors on StatusCode != 200. Logs request and response body on errors.
func (opts Requests) JSON(
	reqDef *http.Request,
	res interface{}) (resp *http.Response, rerr error) {
	resp, logError, err := opts.Do(context.TODO(), reqDef)
	if err != nil {
		rerr = err
		return
	}
	if resp.StatusCode != 200 {
		rerr = logError(fmt.Errorf(`wanted status code 200, got %v`, resp.StatusCode))
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		rerr = logError(err)
		return
	}
	err = json.Unmarshal(b, &res)
	if err != nil {
		rerr = logError(err)
		return
	}
	return
}
