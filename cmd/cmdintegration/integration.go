// Package cmdintegration contains common code for export, validate-config, export-onboard-data. Mainly around configuration.
package cmdintegration

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/iloader"
	"github.com/pinpt/agent.next/rpcdef"
)

type Opts struct {
	Logger       hclog.Logger
	AgentConfig  AgentConfig
	Integrations []Integration
}

type AgentConfig struct {
	CustomerID   string `json:"customer_id"`
	PinpointRoot string `json:"pinpoint_root"`

	// SkipGit is a flag for skipping git repo cloning, ripsrc processing, useful when developing
	SkipGit bool `json:"skip_git"`
	// IntegrationsDir is a custom location of the integrations binaries
	IntegrationsDir string `json:"integrations_dir"`
	// DevUseCompiledIntegrations set to true to use compiled integrations in dev build. They are used by default in prod builds.
	DevUseCompiledIntegrations bool `json:"dev_use_compiled_integrations"`
}

func (s AgentConfig) Locs() (res fsconf.Locs, _ error) {
	root := s.PinpointRoot
	if root == "" {
		v, err := fsconf.DefaultRoot()
		if err != nil {
			return res, err
		}
		root = v
	}
	return fsconf.New(root), nil
}

type Integration struct {
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config"`
}

type Command struct {
	Opts   Opts
	Logger hclog.Logger

	StartTime time.Time
	Locs      fsconf.Locs

	Integrations       map[string]*iloader.Integration
	IntegrationConfigs map[string]Integration
}

func NewCommand(opts Opts) *Command {
	s := &Command{}

	s.Opts = opts
	s.Logger = opts.Logger

	s.StartTime = time.Now()

	var err error
	s.Locs, err = opts.AgentConfig.Locs()
	if err != nil {
		panic(err)
	}

	s.IntegrationConfigs = map[string]Integration{}
	for _, obj := range s.Opts.Integrations {
		s.IntegrationConfigs[obj.Name] = obj
	}

	return s
}

func (s *Command) SetupIntegrations(agent rpcdef.Agent) {
	var integrationNames []string
	for _, integration := range s.Opts.Integrations {
		name := integration.Name
		if name == "" {
			panic("integration name is empty")
		}
		integrationNames = append(integrationNames, name)
	}

	opts := iloader.Opts{}
	opts.Logger = s.Logger
	opts.Locs = s.Locs
	opts.Agent = agent
	opts.IntegrationsDir = s.Opts.AgentConfig.IntegrationsDir
	opts.DevUseCompiledIntegrations = s.Opts.AgentConfig.DevUseCompiledIntegrations
	loader := iloader.New(opts)
	res := loader.Load(integrationNames)
	s.Integrations = res
}
