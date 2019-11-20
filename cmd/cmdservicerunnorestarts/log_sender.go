package cmdservicerunnorestarts

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/go-common/api"
)

// LogSender public interface in case we need to use this outside of this pkg
type logSender struct {
	logger    hclog.Logger
	conf      agentconf.Config
	messageID string
	cmdname   string

	ch     chan []byte
	buf    []byte
	closed chan bool
}

// newLogSender creates an io.Writer that sends logs to elastic, use it with logger.AddWriter()
func newLogSender(logger hclog.Logger, conf agentconf.Config, cmdname, messageID string) io.WriteCloser {
	s := &logSender{}
	s.logger = logger.Named("log-sender")
	s.conf = conf
	s.messageID = messageID
	s.cmdname = cmdname
	s.ch = make(chan []byte, 10000)
	s.closed = make(chan bool)

	maxBufBytes := 10 * 1024 * 1024
	maxDelayBetweenSends := 1 * time.Second

	go func() {
		lastSend := time.Now()
		for b := range s.ch {
			s.buf = append(s.buf, b...)
			if len(s.buf) > maxBufBytes || time.Since(lastSend) > maxDelayBetweenSends {
				s.upload()
				lastSend = time.Now()
				s.buf = nil
			}
		}
		s.closed <- true
	}()

	return s
}

func (s *logSender) upload() {
	perr := func(err error) {
		s.logger.Error("could not upload export log", "err", err)
	}

	url := api.BackendURL(api.EventService, s.conf.Channel)
	client, err := api.NewHTTPAPIClientDefault()
	if err != nil {
		perr(err)
		return
	}

	url += "log/agent/" + s.conf.DeviceID + "/" + s.messageID

	//s.logger.Debug("uploading log", "size", len(s.buf), "url", url)

	// gzip the bytes before sending
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	_, err = gw.Write(s.buf)
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

	api.SetAuthorization(req, s.conf.APIKey)
	api.SetUserAgent(req)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := client.Do(req)
	if err != nil {
		perr(err)
		return
	}

	if resp.StatusCode != http.StatusAccepted {
		buf, _ := ioutil.ReadAll(resp.Body)
		s.logger.Error("error sending log", "err", err, "response", string(buf))
		resp.Body.Close()
		return
	}
	resp.Body.Close()
}

func (s *logSender) Write(b []byte) (int, error) {
	// we must return the number of bytes that were passed in, crashes otherwise
	res := len(b)

	// this should always be json, but check just in case
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return res, fmt.Errorf("backend logs should always be sent in json format. %v", err)
	}
	m["_cmd"] = s.cmdname

	var err error
	b, err = json.Marshal(m)
	if err != nil {
		return res, err
	}

	b = append(b, '\n')

	bCopy := make([]byte, len(b))
	copy(bCopy, b)
	s.ch <- bCopy
	return res, nil
}

func (s *logSender) Close() error {
	close(s.ch)
	<-s.closed
	if len(s.buf) == 0 {
		s.logger.Info("no extra entries in upload log buffer, nothing to upload")
		return nil
	}
	s.upload()
	return nil
}
