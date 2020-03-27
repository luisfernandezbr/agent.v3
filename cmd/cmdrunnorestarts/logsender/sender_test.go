package logsender

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/stretchr/testify/assert"
)

func requestGetString(r *http.Request) (string, error) {
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	resb, err := ioutil.ReadAll(gz)
	if err != nil {
		return "", err
	}
	return string(resb), nil
}

func TestLogSenderBasic1(t *testing.T) {
	mu := sync.Mutex{}
	var results []error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rerr := func(err error) {
			mu.Lock()
			results = append(results, err)
			mu.Unlock()
		}
		res, err := requestGetString(r)
		if err != nil {
			rerr(err)
			return
		}
		if res != `{"msg":"a"}`+"\n" {
			rerr(fmt.Errorf("did not receive the log message, got: %v", res))
			return
		}
		rerr(nil)
	}))
	defer ts.Close()

	opts := Opts{}
	opts.Logger = hclog.New(hclog.DefaultOptions)
	opts.Conf = agentconf.Config{}
	opts.CmdName = "cmd1"
	opts.MessageID = "m1"
	opts.URL = ts.URL
	opts.JSONLineConvert = nilJSONLineConvert

	sender := New(opts)
	_, err := sender.Write([]byte(`{"msg":"a"}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = sender.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no requests made")
	}
	t.Log("waiting for msg on ch")
	for _, err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLogSenderPartialLines1(t *testing.T) {
	mu := sync.Mutex{}
	var results []error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rerr := func(err error) {
			mu.Lock()
			results = append(results, err)
			mu.Unlock()
		}
		res, err := requestGetString(r)
		if err != nil {
			rerr(err)
			return
		}
		if res != `{"msg":"a"}`+"\n" {
			rerr(fmt.Errorf("did not receive the log message, got: %v", res))
			return
		}
		rerr(nil)
	}))
	defer ts.Close()

	opts := Opts{}
	opts.Logger = hclog.New(hclog.DefaultOptions)
	opts.Conf = agentconf.Config{}
	opts.CmdName = "cmd1"
	opts.MessageID = "m1"
	opts.URL = ts.URL
	opts.JSONLineConvert = nilJSONLineConvert

	sender := New(opts)
	wr := func(s string) {
		_, err := sender.Write([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
	}
	wr(`{"msg":"`)
	wr(`a"}` + "\n")
	wr(`{"msg":"`)
	err := sender.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no requests made")
	}
	t.Log("waiting for msg on ch")
	for _, err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}

}

func TestLogSenderPartialLinesHuge(t *testing.T) {
	mu := sync.Mutex{}
	var results []error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rerr := func(err error) {
			mu.Lock()
			results = append(results, err)
			mu.Unlock()
		}
		res, err := requestGetString(r)
		if err != nil {
			rerr(err)
			return
		}
		if res != `{"msg":"a"}`+"\n" {
			rerr(fmt.Errorf("did not receive the log message, got: %v", res))
			return
		}
		rerr(nil)
	}))
	defer ts.Close()

	opts := Opts{}
	opts.Logger = hclog.New(hclog.DefaultOptions)
	opts.Conf = agentconf.Config{}
	opts.CmdName = "cmd1"
	opts.MessageID = "m1"
	opts.URL = ts.URL
	opts.JSONLineConvert = nilJSONLineConvert

	sender := New(opts)
	wr := func(s string) {
		_, err := sender.Write([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
	}
	wr(`{"msg":"a"}` + "\n")
	wr(strings.Repeat("a", maxBufBytes*2))
	err := sender.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no requests made")
	}
	t.Log("waiting for msg on ch")
	for _, err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}

}

func TestLogSenderKeys(t *testing.T) {
	mu := sync.Mutex{}
	var results []error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rerr := func(err error) {
			mu.Lock()
			results = append(results, err)
			mu.Unlock()
		}
		res, err := requestGetString(r)
		if err != nil {
			rerr(err)
			return
		}
		var data map[string]interface{}
		err = json.Unmarshal([]byte(res), &data)
		if err != nil {
			rerr(err)
			return
		}
		delete(data, "timestamp")
		if !assert.Equal(t, map[string]interface{}{
			"k":     "v",
			"level": "info",
			"msg":   "m",
		}, data) {
			rerr(errors.New("got invalid data"))
			return
		}
		rerr(nil)
	}))
	defer ts.Close()

	opts := Opts{}
	opts.Logger = hclog.New(hclog.DefaultOptions)
	opts.Conf = agentconf.Config{}
	opts.CmdName = "cmd1"
	opts.MessageID = "m1"
	opts.URL = ts.URL

	sender := New(opts)

	logOpts := hclog.DefaultOptions
	logOpts.Output = sender
	logOpts.JSONFormat = true
	log := hclog.New(logOpts)
	log.Info("m", "k", "v")

	err := sender.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no requests made")
	}
	t.Log("waiting for msg on ch")
	for _, err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}

}
