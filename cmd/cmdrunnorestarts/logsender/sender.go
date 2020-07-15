// Package logsender contains log Writer that sends the logs to the backend.
package logsender

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	ps "github.com/pinpt/go-common/v10/strings"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/go-common/v10/api"
	pos "github.com/pinpt/go-common/v10/os"
	"github.com/pinpt/httpclient"
)

type Opts struct {
	Logger          hclog.Logger
	Conf            agentconf.Config
	CmdName         string
	MessageID       string
	URL             string
	JSONLineConvert func([]byte) ([]byte, error)
}

// logSenderTimeout is the timeout before giving up on log upload
var logSenderTimeout = pos.Getenv("PP_AGENT_LOG_UPLOAD_TIMEOUT", "2m")

// Sender is a log Writer that send the logs to the backend
type Sender struct {
	opts   Opts
	logger hclog.Logger
	ch     chan []byte
	buf    []byte
	closed chan bool
	client httpclient.Client
}

func newHTTPAPIClientDefault() httpclient.Client {
	dur, err := time.ParseDuration(logSenderTimeout)
	if err != nil {
		panic(fmt.Sprintf("invalid parse duration (%s). error: %s", logSenderTimeout, err))
	}
	cl, err := api.NewHTTPAPIClientDefaultWithTimeout(dur)
	if err != nil {
		panic(err)
	}
	return cl
}

const maxBufBytes = 10 * 1024 * 1024

func nilJSONLineConvert(b []byte) ([]byte, error) {
	return b, nil
}

func defaultJSONLineConvert(b []byte) ([]byte, error) {
	data := map[string]interface{}{}
	err := json.Unmarshal(b, &data)
	if err != nil {
		return nil, err
	}
	if v, ok := data["@level"].(string); ok {
		data["level"] = v
		delete(data, "@level")
	} else {
		return nil, fmt.Errorf("log line is missing @level: %v", string(b))
	}
	if v, ok := data["@message"].(string); ok {
		data["msg"] = v
		delete(data, "@message")
	} else {
		return nil, fmt.Errorf("log line is missing @message: %v", string(b))
	}
	if v, ok := data["@timestamp"].(string); ok {
		data["timestamp"] = v
		delete(data, "@timestamp")
	} else {
		return nil, fmt.Errorf("log line is missing @timestamp: %v", string(b))
	}
	b, err = json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// New creates Sender
func New(opts Opts) *Sender {
	s := &Sender{}
	s.opts = opts
	if s.opts.JSONLineConvert == nil {
		s.opts.JSONLineConvert = defaultJSONLineConvert
	}
	s.logger = opts.Logger.Named("log-sender")
	s.ch = make(chan []byte, 10000)
	s.closed = make(chan bool)
	s.client = newHTTPAPIClientDefault()

	maxDelayBetweenSends := 1 * time.Second

	go func() {
		lastSend := time.Now()
		for b := range s.ch {
			s.buf = append(s.buf, b...)
			if len(s.buf) > maxBufBytes || time.Since(lastSend) > maxDelayBetweenSends {
				s.upload()
				lastSend = time.Now()
			}
		}
		s.closed <- true
	}()

	return s
}

var nl = []byte("\n")

func (s *Sender) upload() {
	perr := func(err error) {
		s.logger.Error("could not upload export log", "err", err)
	}

	url := api.BackendURL(api.EventService, s.opts.Conf.Channel)
	if s.opts.URL != "" {
		url = s.opts.URL
	}

	url = ps.JoinURL(url, "log/agent/"+s.opts.Conf.DeviceID+"/"+s.opts.MessageID)

	//s.logger.Debug("uploading log", "size", len(s.buf), "url", url)

	// only send full lines
	lines := bytes.Split(s.buf, []byte("\n"))
	last := lines[len(lines)-1]
	var toSend [][]byte
	if len(last) == 0 {
		// last was newline, send everything
		toSend = lines
		s.buf = nil
	} else {
		toSend = lines[0 : len(lines)-1]
		toSend = append(toSend, nil) // for newline at the end
		s.buf = last
		if len(last) > maxBufBytes {
			s.logger.Error("attempted to send log line too large", "max", maxBufBytes, "actual", len(last))
			s.buf = nil
		}
	}
	if len(toSend) == 0 {
		s.logger.Debug("nothing to send, only partial log line in buffer")
		return
	}

	var toSend2 [][]byte
	for _, v := range toSend {
		if len(v) == 0 {
			continue
		}
		v2, err := s.opts.JSONLineConvert(v)
		if err != nil {
			s.logger.Error("could not convert keys for log message", "err", err, "v", string(v))
			continue
		}
		toSend2 = append(toSend2, v2)
	}
	toSend = toSend2

	if len(toSend) == 0 {
		s.logger.Debug("nothing to send, all errors converting log message")
		return
	}

	// gzip the bytes before sending
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)

	reqData := bytes.Join(toSend, nl)
	reqData = append(reqData, nl...)

	_, err := gw.Write(reqData)
	if err != nil {
		perr(err)
		return
	}

	err = gw.Close()
	if err != nil {
		perr(err)
		return
	}

	req, err := http.NewRequest(http.MethodPut, url, buf)
	if err != nil {
		perr(err)
		return
	}

	api.SetAuthorization(req, s.opts.Conf.APIKey)
	api.SetUserAgent(req)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := s.client.Do(req)
	if err != nil {
		perr(err)
		return
	}

	if resp.StatusCode != http.StatusAccepted {
		buf, _ := ioutil.ReadAll(resp.Body)
		s.logger.Error("error sending log", "err", err, "response", string(buf), "req", string(reqData))
		resp.Body.Close()
		return
	}
	io.Copy(ioutil.Discard, resp.Body) // must always read body to prevent leak
	resp.Body.Close()
}

// Write implements write interface that can be used by logger.
func (s *Sender) Write(b []byte) (int, error) {
	bCopy := make([]byte, len(b))
	copy(bCopy, b)
	s.ch <- bCopy
	return len(b), nil
}

// Close flushes buffered data and uploads it.
func (s *Sender) Close() error {
	close(s.ch)
	<-s.closed
	if len(s.buf) == 0 {
		s.logger.Info("no extra entries in upload log buffer, nothing to upload")
		return nil
	}
	s.upload()
	return nil
}
