package cmdservicerunnorestarts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/build"

	"github.com/pinpt/agent.next/cmd/cmdintegration"

	"github.com/pinpt/go-common/hash"

	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent.next/cmd/pkg/cmdlogger"
	"github.com/pinpt/agent.next/pkg/deviceinfo"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	isdk "github.com/pinpt/integration-sdk"

	"github.com/pinpt/agent.next/cmd/cmdservicerunnorestarts/crashes"
	"github.com/pinpt/agent.next/cmd/cmdservicerunnorestarts/exporter"
	"github.com/pinpt/agent.next/cmd/cmdservicerunnorestarts/logsender"
	"github.com/pinpt/agent.next/cmd/cmdservicerunnorestarts/updater"
)

type Opts struct {
	Logger cmdlogger.Logger
	// LogLevelSubcommands specifies the log level to pass to sub commands.
	// Pass the same as used for logger.
	// We need it here, because there is no way to get it from logger.
	LogLevelSubcommands hclog.Level
	PinpointRoot        string
}

func Run(ctx context.Context, opts Opts) error {
	s, err := newRunner(opts)
	if err != nil {
		return err
	}
	return s.Run(ctx)
}

type messageHeader struct {
	MessageID string `json:"message_id"`
}

func parseHeader(m map[string]string) (header messageHeader, err error) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &header)
	return
}

type runner struct {
	opts     Opts
	logger   hclog.Logger
	fsconf   fsconf.Locs
	conf     agentconf.Config
	exporter *exporter.Exporter

	agentConfig cmdintegration.AgentConfig
	deviceInfo  deviceinfo.CommonInfo

	logSender *logsender.Sender

	onboardingInProgress int64
}

func newRunner(opts Opts) (*runner, error) {
	s := &runner{}
	s.opts = opts
	s.fsconf = fsconf.New(opts.PinpointRoot)

	var err error
	s.conf, err = agentconf.Load(s.fsconf.Config2)
	if err != nil {
		return nil, err
	}
	s.agentConfig = s.getAgentConfig()
	s.deviceInfo = s.getDeviceInfoOpts()

	s.logSender = logsender.New(s.opts.Logger, s.conf, "service-run", hash.Values(time.Now()))
	s.logger = s.opts.Logger.AddWriter(s.logSender)

	return s, nil
}

type closefunc func()

func (s *runner) close() {
	if err := s.logSender.Close(); err != nil {
		s.logger.Error("error closing log sender", "err", err)
	}
}

func (s *runner) Run(ctx context.Context) error {
	defer func() {
		s.close()
	}()

	s.logger.Info("Starting service", "pinpoint-root", s.opts.PinpointRoot)

	s.logger.Info("Config", "version", os.Getenv("PP_AGENT_VERSION"), "commit", os.Getenv("PP_AGENT_COMMIT"), "pinpoint-root", s.opts.PinpointRoot, "integrations-dir", s.conf.IntegrationsDir)

	if build.IsProduction() &&
		(runtime.GOOS == "linux" || runtime.GOOS == "windows") &&
		os.Getenv("PP_AGENT_UPDATE_ENABLED") != "" {
		version := os.Getenv("PP_AGENT_UPDATE_VERSION")
		if version != "" {
			oldVersion, err := s.updateTo(version)
			if err != nil {
				return fmt.Errorf("Could not self-update: %v", err)
			}
			if oldVersion != version {
				s.logger.Info("Updated the agent, restarting...")
				return nil
			}
		} else {
			upd := updater.New(s.logger, s.fsconf, s.conf)
			err := upd.DownloadIntegrationsIfMissing()
			if err != nil {
				return fmt.Errorf("Could not download integration binaries: %v", err)
			}
		}
	}

	err := crashes.New(s.logger, s.fsconf, s.sendEventAppendingDeviceInfoDefault).Send()
	if err != nil {
		return fmt.Errorf("could not send crashes, err: %v", err)
	}

	err = s.sendEnabled(ctx)
	if err != nil {
		return fmt.Errorf("could not send enabled request, err: %v", err)
	}

	err = s.sendStart(context.Background())
	if err != nil {
		return fmt.Errorf("could not send start event, err: %v", err)
	}

	s.exporter, err = exporter.New(exporter.Opts{
		Logger:              s.logger,
		LogLevelSubcommands: s.opts.LogLevelSubcommands,
		PinpointRoot:        s.opts.PinpointRoot,
		Conf:                s.conf,
		FSConf:              s.fsconf,
		PPEncryptionKey:     s.conf.PPEncryptionKey,
		AgentConfig:         s.agentConfig,
		IntegrationsDir:     s.fsconf.Integrations,
	})
	if err != nil {
		return fmt.Errorf("could not initialize exporter, err: %v", err)
	}

	go func() {
		s.sendPings()
	}()

	go func() {
		s.exporter.Run()
	}()

	{
		close, err := s.handleUpdateEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling update requests, err: %v", err)
		}
		defer close()
	}
	{
		close, err := s.handleIntegrationEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling integration requests, err: %v", err)
		}
		defer close()
	}
	{
		close, err := s.handleOnboardingEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling onboarding requests, err: %v", err)
		}

		defer close()
	}
	{
		close, err := s.handleExportEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling export requests, err: %v", err)
		}
		defer close()
	}

	/*
		if os.Getenv("PP_AGENT_SERVICE_TEST_MOCK") != "" {
			s.logger.Info("PP_AGENT_SERVICE_TEST_MOCK passed, running test mock export")
			err := s.runTestMockExport()
			if err != nil {
				return fmt.Errorf("could not export mock, err: %v", err)
			}
		}*/

	s.logger.Info("waiting for requests...")

	block := make(chan bool)
	<-block

	return nil
}

func (s *runner) sendEnabled(ctx context.Context) error {

	data := agent.Enabled{
		CustomerID: s.conf.CustomerID,
		UUID:       s.conf.DeviceID,
	}
	data.Success = true
	data.Error = nil
	data.Data = nil

	s.deviceInfo.AppendCommonInfo(&data)

	publishEvent := event.PublishEvent{
		Object: &data,
		Headers: map[string]string{
			"uuid": s.conf.DeviceID,
		},
	}

	err := event.Publish(ctx, publishEvent, s.conf.Channel, s.conf.APIKey)
	if err != nil {
		panic(err)
	}

	return nil
}

type modelFactory struct {
}

func (f *modelFactory) New(name datamodel.ModelNameType) datamodel.Model {
	return isdk.New(name)
}

var factory action.ModelFactory = &modelFactory{}

func (s *runner) newSubConfig(topic string) action.Config {
	errorsChan := make(chan error, 1)
	go func() {
		for err := range errorsChan {
			s.logger.Error("error in integration requests", "err", err)
		}
	}()
	return action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   topic,
		Errors:  errorsChan,
		Headers: map[string]string{
			"customer_id": s.conf.CustomerID,
			"uuid":        s.conf.DeviceID,
		},
		Offset: "earliest",
	}
}

func (s *runner) sendPings() {
	ctx := context.Background()
	s.sendPing(ctx) // always send ping immediately upon start
	for {
		select {
		case <-time.After(30 * time.Second):
			err := s.sendPing(ctx)
			if err != nil {
				s.logger.Error("could not send ping", "err", err.Error())
			}
		}
	}
}

func (s *runner) sendStart(ctx context.Context) error {
	agentEvent := &agent.Start{
		Type:    agent.StartTypeStart,
		Success: true,
	}
	return s.sendEvent(ctx, agentEvent, "", nil)
}

func (s *runner) sendStop(ctx context.Context) error {
	agentEvent := &agent.Stop{
		Type:    agent.StopTypeStop,
		Success: true,
	}
	return s.sendEvent(ctx, agentEvent, "", nil)
}

func (s *runner) sendPing(ctx context.Context) error {
	ev := &agent.Ping{
		Type:    agent.PingTypePing,
		Success: true,
	}
	onboardingInProgress := atomic.LoadInt64(&s.onboardingInProgress)
	ev.Onboarding = onboardingInProgress != 0

	if s.exporter.IsRunning() {
		ev.State = agent.PingStateExporting
		ev.Exporting = true
	} else {
		ev.State = agent.PingStateIdle
		ev.Exporting = false
	}
	return s.sendEvent(ctx, s.getPing(), "", nil)
}

func (s *runner) getPing() *agent.Ping {
	ev := &agent.Ping{
		Type:    agent.PingTypePing,
		Success: true,
	}
	onboardingInProgress := atomic.LoadInt64(&s.onboardingInProgress)
	ev.Onboarding = onboardingInProgress != 0

	if s.exporter.IsRunning() {
		ev.State = agent.PingStateExporting
		ev.Exporting = true
	} else {
		ev.State = agent.PingStateIdle
		ev.Exporting = false
	}
	return ev
}

func (s *runner) sendEvent(ctx context.Context, agentEvent datamodel.Model, jobID string, extraHeaders map[string]string) error {
	s.deviceInfo.AppendCommonInfo(agentEvent)
	headers := map[string]string{
		"uuid":        s.conf.DeviceID,
		"customer_id": s.conf.CustomerID,
	}
	if jobID != "" {
		headers["job_id"] = jobID
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}
	e := event.PublishEvent{
		Object:  agentEvent,
		Headers: headers,
	}
	return event.Publish(ctx, e, s.conf.Channel, s.conf.APIKey)
}

func (s *runner) sendEventAppendingDeviceInfo(ctx context.Context, event datamodel.Model, jobID string, extraHeaders map[string]string) error {
	s.deviceInfo.AppendCommonInfo(event)
	return s.sendEvent(ctx, event, jobID, extraHeaders)
}

func (s *runner) sendEventAppendingDeviceInfoDefault(ctx context.Context, event datamodel.Model) error {
	s.deviceInfo.AppendCommonInfo(event)
	return s.sendEvent(ctx, event, "", nil)
}

func (s *runner) getAgentConfig() (res cmdintegration.AgentConfig) {
	res.CustomerID = s.conf.CustomerID
	res.PinpointRoot = s.opts.PinpointRoot
	res.IntegrationsDir = s.conf.IntegrationsDir
	res.Backend.Enable = true
	return
}

func (s *runner) getDeviceInfoOpts() deviceinfo.CommonInfo {
	return deviceinfo.CommonInfo{
		CustomerID: s.conf.CustomerID,
		Root:       s.opts.PinpointRoot,
		SystemID:   s.conf.SystemID,
		DeviceID:   s.conf.DeviceID,
	}
}
