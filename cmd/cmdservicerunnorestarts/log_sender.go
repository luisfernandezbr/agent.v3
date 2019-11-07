package cmdservicerunnorestarts

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/go-common/api"
)

type exportLogSender struct {
	logger      hclog.Logger
	conf        agentconf.Config
	exportJobID string

	ch     chan []byte
	buf    []byte
	closed chan bool
}

func newExportLogSender(logger hclog.Logger, conf agentconf.Config, exportJobID string) *exportLogSender {
	s := &exportLogSender{}
	s.logger = logger.Named("log-sender")
	s.conf = conf
	s.exportJobID = exportJobID

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

func (s *exportLogSender) upload() {
	perr := func(err error) {
		s.logger.Error("could not upload export log", "err", err)
	}

	url := api.BackendURL(api.EventService, s.conf.Channel)
	client, err := api.NewHTTPAPIClientDefault()
	if err != nil {
		perr(err)
		return
	}

	url += "log/agent/" + s.conf.DeviceID + "/" + s.exportJobID

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

func (s *exportLogSender) Write(b []byte) (n int, _ error) {
	bCopy := make([]byte, len(b))
	copy(bCopy, b)
	s.ch <- bCopy
	return len(b), nil
}

func (s *exportLogSender) FlushAndClose() error {
	close(s.ch)
	<-s.closed
	if len(s.buf) == 0 {
		s.logger.Info("no extra entries in upload log buffer, nothing to upload")
		return nil
	}
	s.upload()
	return nil
}
