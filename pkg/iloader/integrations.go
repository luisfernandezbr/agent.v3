package iloader

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/integrationid"
	"github.com/pinpt/agent.next/rpcdef"
)

type Opts struct {
	Logger         hclog.Logger
	Locs           fsconf.Locs
	AgentDelegates func(in integrationid.ID) rpcdef.Agent

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

func (s *Loader) Load(ins []integrationid.ID) (map[string]*Integration, error) {
	s.logger.Info("Loading integrations", "ins", fmt.Sprintf("%+v", ins))

	type intStruct struct {
		id          string
		integration *Integration
	}
	res := make(chan intStruct)
	var rerr error
	var errMu sync.Mutex

	go func() {
		wg := sync.WaitGroup{}
		for _, in := range ins {
			wg.Add(1)
			in := in
			go func() {
				defer wg.Done()
				one, err := s.load(in)
				if err != nil {
					errMu.Lock()
					rerr = err
					errMu.Unlock()
					return
				}
				res <- intStruct{
					id:          in.String(),
					integration: one,
				}
			}()
		}
		wg.Wait()
		close(res)
	}()
	loaded := map[string]*Integration{}
	for each := range res {
		loaded[each.id] = each.integration
	}
	return loaded, rerr
}

func (s *Loader) load(in integrationid.ID) (*Integration, error) {
	opts := IntegrationOpts{}
	opts.Logger = s.opts.Logger
	opts.Agent = s.opts.AgentDelegates(in)
	opts.Name = in.Name
	opts.Locs = s.locs
	opts.DevUseCompiledIntegrations = s.opts.DevUseCompiledIntegrations
	return NewIntegration(opts)
}
