package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/hashicorp/go-hclog"
)

type Opts struct {
	Logger  hclog.Logger
	Client  *http.Client
	LogBody bool
}

func requestExtractBody(req http.Request) (res *http.Request, reqBody []byte, rerr error) {
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rerr = err
		return
	}

	res, err = http.NewRequest(req.Method, req.URL.String(), bytes.NewReader(b))
	if err != nil {
		rerr = err
		return
	}

	return res, b, nil
}

func AppendURL(p1, p2 string) string {
	return strings.TrimSuffix(p1, "/") + "/" + p2
}

func JSON(
	ctx context.Context,
	opts Opts,
	reqDef *http.Request,
	res interface{}) (resp *http.Response, rerr error) {

	logger := opts.Logger

	u := reqDef.URL.String()
	logger.Debug("request", "url", u)

	req := reqDef.WithContext(ctx)
	var err error
	resp, err = opts.Client.Do(req)
	if err != nil {
		rerr = err
		return
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		rerr = err
		return
	}
	ferr := func(err error) {
		logger.Debug("request failed", "url", u, "body", string(b), "err", err.Error())
		rerr = fmt.Errorf("request failed url: %v err: %v", u, err)
	}
	if resp.StatusCode != 200 {
		ferr(fmt.Errorf(`wanted status code 200, got %v`, resp.StatusCode))
		return
	}
	err = json.Unmarshal(b, &res)
	if err != nil {
		ferr(fmt.Errorf(`could not parse response as json %v`, err))
		return
	}
	return resp, nil
}
