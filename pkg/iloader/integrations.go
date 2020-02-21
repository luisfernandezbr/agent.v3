package iloader

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/expin"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/rpcdef"
)

type Opts struct {
	Logger         hclog.Logger
	Locs           fsconf.Locs
	AgentDelegates func(ind expin.Export) rpcdef.Agent

	// IntegrationsDir is a custom location of the integrations binaries
	IntegrationsDir string
	// DevUseCompiledIntegrations set to true to use compiled integrations in dev build. They are used by default in prod builds.
	DevUseCompiledIntegrations bool
}

type Loader struct {
	opts   Opts
	logger hclog.Logger
	locs   fsconf.Locs
}

func New(opts Opts) *Loader {
	if opts.Logger == nil || opts.Locs.Root == "" || opts.AgentDelegates == nil || opts.IntegrationsDir == "" {
		panic("provide all opts")
	}
	s := &Loader{}
	s.opts = opts
	s.logger = opts.Logger
	s.locs = opts.Locs
	return s
}

func (s *Loader) Load(exports []expin.Export) (res map[expin.Export]*Integration, _ error) {
	s.logger.Info("Loading integrations", "expin", fmt.Sprintf("%+v", exports))

	res = map[expin.Export]*Integration{}
	var resMu sync.Mutex
	var rerr error
	var errMu sync.Mutex

	wg := sync.WaitGroup{}
	for _, export := range exports {
		wg.Add(1)
		export := export
		go func() {
			defer wg.Done()
			in, err := s.load(export)
			if err != nil {
				errMu.Lock()
				rerr = err
				errMu.Unlock()
				return
			}
			resMu.Lock()
			res[export] = in
			resMu.Unlock()
		}()
	}
	wg.Wait()
	if rerr != nil {
		return nil, rerr
	}
	return
}

func (s *Loader) load(export expin.Export) (*Integration, error) {
	opts := IntegrationOpts{}
	opts.Logger = s.opts.Logger
	opts.Agent = s.opts.AgentDelegates(export)
	opts.Export = export
	opts.Locs = s.locs
	opts.IntegrationsDir = s.opts.IntegrationsDir
	opts.DevUseCompiledIntegrations = s.opts.DevUseCompiledIntegrations
	return NewIntegration(opts)
}
