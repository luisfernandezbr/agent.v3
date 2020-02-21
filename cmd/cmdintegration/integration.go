// Package cmdintegration contains common code for export, validate-config, export-onboard-data. Mainly around configuration.
package cmdintegration

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/expin"
	"github.com/pinpt/agent/pkg/structmarshal"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/deviceinfo"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/iloader"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"
)

type Opts struct {
	Logger       hclog.Logger
	AgentConfig  AgentConfig
	Integrations []inconfig.Integration
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
	// When created from passed args Export, ExportConfig and OauthRefreshToken are set. ILoader is set later before running.

	Export       expin.Export
	ExportConfig rpcdef.ExportConfig
	// OAuthRefreshTokens contains refresh token for integrations
	// using OAuth. These are allow getting new access tokens using
	// pinpoint backend. Do not pass them to integrations, these are handled in agent instead.
	OauthRefreshToken string

	ILoader *iloader.Integration
}

type Command struct {
	Opts   Opts
	Logger hclog.Logger

	StartTime time.Time
	Locs      fsconf.Locs

	Integrations map[expin.Export]Integration

	EnrollConf agentconf.Config
	Deviceinfo deviceinfo.CommonInfo

	integrationsDir            string
	devUseCompiledIntegrations bool
}

func NewCommand(opts Opts) (*Command, error) {
	s := &Command{}

	s.Opts = opts
	s.Logger = opts.Logger
	s.StartTime = time.Now()

	s.Logger.Debug("starting command", "pinpoint-root", opts.AgentConfig.PinpointRoot, "integrations-dir", opts.AgentConfig.IntegrationsDir)

	var err error
	s.Locs, err = opts.AgentConfig.Locs()
	if err != nil {
		return nil, err
	}

	s.integrationsDir = opts.AgentConfig.IntegrationsDir
	s.devUseCompiledIntegrations = opts.AgentConfig.DevUseCompiledIntegrations
	if s.integrationsDir == "" {
		s.integrationsDir = s.Locs.IntegrationsDefaultDir
	} else {
		s.devUseCompiledIntegrations = true // force the use of compiled integrations if integrations dir is provided
	}

	s.Logger.Debug("resolved config", "PinpointRoot", s.Locs.Root, "IntegrationsDir", s.integrationsDir)

	err = os.MkdirAll(s.Locs.State, 0777)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(s.Locs.Temp, 0777)
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

func (s *Command) OnlyIntegration() Integration {
	if len(s.Integrations) == 1 {
		for _, v := range s.Integrations {
			return v
		}
	}
	panic("cmdintegration.OnlyIntegration called when s.Integrations != 1")
}

func (s *Command) setupConfig() error {

	s.Integrations = map[expin.Export]Integration{}

	for i, obj := range s.Opts.Integrations {
		id := obj.ID
		if id == "" {
			id = strconv.Itoa(i)
		}
		def := obj.IntegrationDef()
		exp := expin.NewExport(id, def)

		in := Integration{}
		in.Export = exp

		ec := rpcdef.ExportConfig{}
		ec.Pinpoint.CustomerID = s.Opts.AgentConfig.CustomerID

		if refresh, ok := obj.Config["refresh_token"].(string); ok && refresh != "" {
			in.OauthRefreshToken = refresh
			ec.UseOAuth = true
		}
		delete(obj.Config, "refresh_token")

		if err := structmarshal.StructToStruct(obj, &ec.Integration); err != nil {
			return err
		}

		in.ExportConfig = ec
		s.Integrations[exp] = in

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
	agentDelegates func(ind expin.Export) rpcdef.Agent) error {

	if agentDelegates == nil {
		agentDelegates = AgentDelegateMinFactory(s.Logger, s)
	}

	var ins []expin.Export
	for _, in := range s.Integrations {
		ins = append(ins, in.Export)
	}

	opts := iloader.Opts{}
	opts.Logger = s.Logger
	opts.Locs = s.Locs
	opts.AgentDelegates = agentDelegates
	opts.IntegrationsDir = s.integrationsDir
	opts.DevUseCompiledIntegrations = s.devUseCompiledIntegrations
	loader := iloader.New(opts)
	res, err := loader.Load(ins)
	if err != nil {
		return err
	}

	for key, v := range res {
		in := s.Integrations[key]
		in.ILoader = v
		s.Integrations[key] = in
	}

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
		if s.Opts.AgentConfig.Backend.Enable {
			data := &agent.Crash{
				Data:      &panicOut,
				Type:      agent.CrashTypeCrash,
				Component: "integration/" + integration.Export.IntegrationDef.String(),
			}
			date.ConvertToModel(time.Now(), &data.CrashDate)
			if err := s.sendEvent(data); err != nil {
				s.Logger.Error("error sending agent.Crash to backend", "err", err)
			}
		}
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

func (s *Command) SendPauseEvent(export expin.Export, msg string, resumeDate time.Time) error {
	s.Logger.Info("pausing integration due to throttling", "msg", msg, "integration", export.String(), "duration", resumeDate.Sub(time.Now()).String())

	data := &agent.Pause{
		Data:        &msg,
		Type:        agent.PauseTypePause,
		Integration: export.String(),
		JobID:       s.Opts.AgentConfig.Backend.ExportJobID,
	}
	date.ConvertToModel(resumeDate, &data.ResumeDate)
	date.ConvertToModel(time.Now(), &data.EventDate)
	if err := s.sendEvent(data); err != nil {
		return fmt.Errorf("error sending agent.Pause to backend. err %v", err)
	}
	return nil
}
func (s *Command) SendResumeEvent(export expin.Export, msg string) error {
	s.Logger.Info("continue with integration after throttling", "msg", msg, "integration", export.String())

	data := &agent.Resume{
		Data:        &msg,
		Type:        agent.ResumeTypeResume,
		Integration: export.String(),
		JobID:       s.Opts.AgentConfig.Backend.ExportJobID,
	}
	date.ConvertToModel(time.Now(), &data.EventDate)
	if err := s.sendEvent(data); err != nil {
		return fmt.Errorf("error sending agent.Resume to backend. err %v", err)
	}

	return nil
}

func (s *Command) sendEvent(data datamodel.Model) error {
	if !s.Opts.AgentConfig.Backend.Enable {
		return nil
	}
	s.Deviceinfo.AppendCommonInfo(data)
	publishEvent := event.PublishEvent{
		Object: data,
		Headers: map[string]string{
			"uuid":        s.EnrollConf.DeviceID,
			"customer_id": s.EnrollConf.CustomerID,
			"job_id":      s.Opts.AgentConfig.Backend.ExportJobID,
		},
	}
	return event.Publish(context.Background(), publishEvent, s.EnrollConf.Channel, s.EnrollConf.APIKey)
}
