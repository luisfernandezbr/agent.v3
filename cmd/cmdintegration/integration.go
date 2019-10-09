// Package cmdintegration contains common code for export, validate-config, export-onboard-data. Mainly around configuration.
package cmdintegration

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/iloader"
	"github.com/pinpt/agent.next/rpcdef"
)

type Opts struct {
	Logger hclog.Logger

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

	Backend struct {
		// Enable enables calls to pinpoint backend. It is disabled by default, but is required for the following features:
		// - sending progress data to backend
		// - using OAuth with refresh token
		Enable bool `json:"enable"`

		// ExportJobID is passed to backend in progress event
		ExportJobID string `json:"export_job_id"`
	} `json:"backend"`
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

	Integrations  map[string]*iloader.Integration
	ExportConfigs map[string]rpcdef.ExportConfig

	// OAuthRefreshTokens contains refresh token for integrations
	// using OAuth. These are allow getting new access tokens using
	// pinpoint backend. Do not pass them to integrations, these are handled in agent instead.
	OAuthRefreshTokens map[string]string

	EnrollConf agentconf.Config
}

func NewCommand(opts Opts) (*Command, error) {
	s := &Command{}

	s.Opts = opts
	s.Logger = opts.Logger

	s.StartTime = time.Now()

	var err error
	s.Locs, err = opts.AgentConfig.Locs()
	if err != nil {
		return nil, err
	}

	err = s.setupConfig()
	if err != nil {
		return nil, err
	}

	if opts.AgentConfig.Backend.Enable {
		var err error
		s.EnrollConf, err = agentconf.Load(s.Locs.Config2)
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Command) setupConfig() error {
	s.ExportConfigs = map[string]rpcdef.ExportConfig{}
	s.OAuthRefreshTokens = map[string]string{}

	for _, obj := range s.Opts.Integrations {
		name := obj.Name
		if len(obj.Config) == 0 {
			return fmt.Errorf("empty config for integration: %v", name)
		}

		ec := rpcdef.ExportConfig{}
		ec.Pinpoint.CustomerID = s.Opts.AgentConfig.CustomerID
		ec.Integration = obj.Config

		refreshToken, _ := obj.Config["oauth_refresh_token"].(string)
		if refreshToken != "" {
			s.OAuthRefreshTokens[name] = refreshToken
			ec.UseOAuth = true
			// do not pass oauth_refresh_token to integration
			// integrations should use
			// NewAccessToken() to get access token instead
			delete(ec.Integration, "oauth_refresh_token")
		}

		s.ExportConfigs[obj.Name] = ec

	}
	return nil
}

func copyMap(data map[string]interface{}) map[string]interface{} {
	res := map[string]interface{}{}
	for k, v := range data {
		res[k] = v
	}
	return res
}

func (s *Command) SetupIntegrations(
	agentDelegates func(integrationName string) rpcdef.Agent) error {

	if agentDelegates == nil {
		agentDelegates = AgentDelegateMinFactory(s)
	}

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
	opts.AgentDelegates = agentDelegates
	opts.IntegrationsDir = s.Opts.AgentConfig.IntegrationsDir
	opts.DevUseCompiledIntegrations = s.Opts.AgentConfig.DevUseCompiledIntegrations
	loader := iloader.New(opts)
	res, err := loader.Load(integrationNames)
	if err != nil {
		return err
	}
	s.Integrations = res

	go func() {
		s.CaptureShutdown()
	}()

	return nil
}

func (s *Command) CloseOnlyIntegrationAndHandlePanic(integration *iloader.Integration) error {
	panicOut, err := integration.CloseAndDetectPanic()
	if panicOut != "" {
		// This is already printed in integration logs, but easier to debug if it's repeated in stdout.
		fmt.Println("Panic in integration")
		fmt.Println(panicOut)
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *Command) CaptureShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	plugin.CleanupClients()
	os.Exit(1)
}
