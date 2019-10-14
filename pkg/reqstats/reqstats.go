// Package reqstats adds logging and statistics to integration requests
package reqstats

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/httpdefaults"
)

type Clients struct {
	Default     *http.Client
	TLSInsecure *http.Client
}

type Opts struct {
	Logger hclog.Logger
	// TLSInsecureSkipVerify to disable tls cert checks when integration uses TLSInsecure() client
	TLSInsecureSkipVerify bool
}

type ClientManager struct {
	opts   Opts
	logger hclog.Logger

	Clients Clients

	totalRequests *int64
}

func int64p() *int64 {
	var v int64
	return &v
}

func New(opts Opts) *ClientManager {
	if opts.Logger == nil {
		panic("provide logger")
	}

	s := &ClientManager{}
	s.opts = opts
	s.logger = opts.Logger.Named("reqstats")
	s.totalRequests = int64p()

	{
		c := &http.Client{}
		transport := httpdefaults.DefaultTransport()
		c.Transport = s.wrapRoundTripper(transport)
		s.Clients.Default = c
	}

	if !s.opts.TLSInsecureSkipVerify {
		s.Clients.TLSInsecure = s.Clients.Default
	} else {
		c := &http.Client{}
		transport := httpdefaults.DefaultTransport()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		c.Transport = s.wrapRoundTripper(transport)
		s.Clients.TLSInsecure = c
	}

	return s
}

func (s ClientManager) PrintStats() string {
	var res []string
	l := func(args ...interface{}) {
		res = append(res, fmt.Sprintln(args...))
	}
	l("HTTP Requests Stats")
	l("Total requests:", *s.totalRequests)
	return strings.Join(res, "")
}

func (s *ClientManager) wrapRoundTripper(rt http.RoundTripper) http.RoundTripper {
	fn := func(req *http.Request) (*http.Response, error) {
		start := time.Now()
		l := s.logger.With("url", req.URL.String())
		atomic.AddInt64(s.totalRequests, 1)
		l.Debug("req start")
		res, err := rt.RoundTrip(req)
		sec := fmt.Sprintf("%.1f", time.Since(start).Seconds())
		if err != nil {
			l.Debug("req end with err", "err", err, "sec", sec)
			return res, err
		}
		l.Debug("req end", "code", res.StatusCode, "sec", sec)
		return res, err
	}
	return roundTripperFn{Fn: fn}
}

type roundTripperFn struct {
	Fn func(*http.Request) (*http.Response, error)
}

func (s roundTripperFn) RoundTrip(req *http.Request) (*http.Response, error) {
	return s.Fn(req)
}
