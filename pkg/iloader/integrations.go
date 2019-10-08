package iloader

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/rpcdef"
)

type Opts struct {
	Logger         hclog.Logger
	Locs           fsconf.Locs
	AgentDelegates func(integrationName string) rpcdef.Agent

	// IntegrationsDir is a custom location of the integrations binaries
	IntegrationsDir string `json:"integrations_dir"`
	// DevUseCompiledIntegrations set to true to use compiled integrations in dev build. They are used by default in prod builds.
	DevUseCompiledIntegrations bool `json:"dev_use_compiled_integrations"`
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
	}
	return s
}

func (s *Loader) Load(names []string) (map[string]*Integration, error) {
	s.logger.Info("Loading integrations", "names", names)

	res := make(chan *Integration)
	var rerr error
	var errMu sync.Mutex

	go func() {
		wg := sync.WaitGroup{}
		for _, name := range names {
			wg.Add(1)
			name := name
			go func() {
				defer wg.Done()
				one, err := s.load(name)
				if err != nil {
					errMu.Lock()
					rerr = err
					errMu.Unlock()
					return
				}
				res <- one
			}()
		}
		wg.Wait()
		close(res)
	}()
	loaded := map[string]*Integration{}
	for integration := range res {
		loaded[integration.Name()] = integration
	}
	return loaded, rerr
}

func (s *Loader) load(integrationName string) (*Integration, error) {
	opts := IntegrationOpts{}
	opts.Logger = s.opts.Logger
	opts.Agent = s.opts.AgentDelegates(integrationName)
	opts.Name = integrationName
	opts.Locs = s.locs
	opts.DevUseCompiledIntegrations = s.opts.DevUseCompiledIntegrations
	return NewIntegration(opts)
}
