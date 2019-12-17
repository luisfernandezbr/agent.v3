package iloader

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/integrationid"
	"github.com/pinpt/agent/rpcdef"
)

type Opts struct {
	Logger         hclog.Logger
	Locs           fsconf.Locs
	AgentDelegates func(in integrationid.ID) rpcdef.Agent

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
	if opts.Logger == nil || opts.Locs.Root == "" || opts.AgentDelegates == nil {
		panic("provide all opts")
	}
	s := &Loader{}
	s.opts = opts
	s.logger = opts.Logger
	s.locs = opts.Locs
	if opts.IntegrationsDir != "" {
		s.locs.Integrations = opts.IntegrationsDir
		s.opts.DevUseCompiledIntegrations = true // force the use of compiled integrations if integrations dir is provided
	}
	return s
}

func (s *Loader) Load(ids []integrationid.ID) (map[integrationid.ID]*Integration, error) {
	s.logger.Info("Loading integrations", "ids", fmt.Sprintf("%+v", ids))

	res := make(chan *Integration)
	var rerr error
	var errMu sync.Mutex

	go func() {
		wg := sync.WaitGroup{}
		for _, id := range ids {
			wg.Add(1)
			id := id
			go func() {
				defer wg.Done()
				in, err := s.load(id)
				if err != nil {
					errMu.Lock()
					rerr = err
					errMu.Unlock()
					return
				}
				res <- in
			}()
		}
		wg.Wait()
		close(res)
	}()

	loaded := map[integrationid.ID]*Integration{}
	for in := range res {
		loaded[in.ID] = in
	}
	return loaded, rerr
}

func (s *Loader) load(id integrationid.ID) (*Integration, error) {
	opts := IntegrationOpts{}
	opts.Logger = s.opts.Logger
	opts.Agent = s.opts.AgentDelegates(id)
	opts.ID = id
	opts.Locs = s.locs
	opts.DevUseCompiledIntegrations = s.opts.DevUseCompiledIntegrations
	return NewIntegration(opts)
}
