package cmdrunnorestarts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/aevent"
	"github.com/pinpt/agent/pkg/build"

	"github.com/pinpt/agent/cmd/cmdintegration"

	"github.com/pinpt/go-common/hash"

	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent/cmd/pkg/cmdlogger"
	"github.com/pinpt/agent/pkg/deviceinfo"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	isdk "github.com/pinpt/integration-sdk"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/crashes"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/exporter"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/logsender"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/updater"
)

type Opts struct {
	Logger cmdlogger.Logger
	// LogLevelSubcommands specifies the log level to pass to sub commands.
	// Pass the same as used for logger.
	// We need it here, because there is no way to get it from logger.
	LogLevelSubcommands hclog.Level
	PinpointRoot        string

	AgentConf agentconf.Config
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

	for _, dir := range s.fsconf.CleanupDirs {
		err := os.RemoveAll(dir)
		if err != nil {
			return nil, err
		}
	}

	s.conf = opts.AgentConf

	s.agentConfig = s.getAgentConfig()
	s.deviceInfo = s.getDeviceInfoOpts()

	{
		opts := logsender.Opts{}
		opts.Logger = s.opts.Logger
		opts.Conf = s.conf
		opts.CmdName = "run"
		opts.MessageID = hash.Values(time.Now())
		s.logSender = logsender.New(opts)
	}
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
	var closers []func()
	// do not use defer, to avoid indefinite timeouts in case of panics
	closers = append(closers, s.close)

	s.logger.Info("Starting service", "pinpoint-root", s.opts.PinpointRoot)

	s.logger.Info("Config", "version", os.Getenv("PP_AGENT_VERSION"), "commit", os.Getenv("PP_AGENT_COMMIT"), "pinpoint-root", s.opts.PinpointRoot, "integrations-dir", s.conf.IntegrationsDir, "uuid", s.opts.AgentConf.DeviceID)

	s.logger.Debug("Debug log level enabled")

	if build.IsProduction() &&
		(runtime.GOOS == "linux" || runtime.GOOS == "windows") {
		toVersion := os.Getenv("PP_AGENT_UPDATE_VERSION")
		if toVersion != "" && toVersion != "dev" {
			_, updated, err := s.updateTo(toVersion)
			if err != nil {
				return fmt.Errorf("Could not self-update: %v", err)
			}
			if updated {
				s.logger.Info("Updated the agent, restarting...")
				return nil
			}
		} else {
			fromVersion := os.Getenv("PP_AGENT_VERSION")
			// skip auto integration download for test builds
			if fromVersion != "test" {
				upd := updater.New(s.logger, s.fsconf, s.conf)
				err := upd.DownloadIntegrationsIfMissing()
				if err != nil {
					return fmt.Errorf("Could not download integration binaries: %v", err)
				}
			}

		}
	}

	err := crashes.New(s.logger, s.fsconf, s.sendEventAppendingDeviceInfoDefault).Send()
	if err != nil {
		return fmt.Errorf("could not send crashes, err: %v", err)
	}

	s.logger.Info("Sending enabled event")

	err = s.sendEnabled(ctx)
	if err != nil {
		return fmt.Errorf("could not send enabled request, err: %v", err)
	}

	s.logger.Info("Sent enabled event")

	err = s.sendStart(ctx)
	if err != nil {
		return fmt.Errorf("could not send start event, err: %v", err)
	}
	defer func() {
		if err := s.sendStop(ctx); err != nil {
			s.logger.Error("Could not send stop event", err, "err")
			return
		}
	}()
	s.exporter, err = exporter.New(exporter.Opts{
		Logger:              s.logger,
		LogLevelSubcommands: s.opts.LogLevelSubcommands,
		PinpointRoot:        s.opts.PinpointRoot,
		Conf:                s.conf,
		FSConf:              s.fsconf,
		PPEncryptionKey:     s.conf.PPEncryptionKey,
		AgentConfig:         s.agentConfig,
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
		closers = append(closers, close)
	}
	{
		close, err := s.handleIntegrationEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling integration requests, err: %v", err)
		}
		closers = append(closers, close)
	}
	{
		close, err := s.handleOnboardingEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling onboarding requests, err: %v", err)
		}

		closers = append(closers, close)
	}
	{
		close, err := s.handleExportEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling export requests, err: %v", err)
		}
		closers = append(closers, close)
	}
	{
		close, err := s.handleCancelEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling cancel requests, err: %v", err)
		}
		closers = append(closers, close)
	}
	{
		close, err := s.handleMutationEvents(ctx)
		if err != nil {
			return fmt.Errorf("error handling mutation requests, err: %v", err)
		}
		closers = append(closers, close)
	}

	finishMain := make(chan bool, 1)
	{
		close, err := s.handleUninstallEvents(ctx, finishMain)
		if err != nil {
			return fmt.Errorf("error handling uninstall requests, err: %v", err)
		}
		closers = append(closers, close)
	}

	// go func() {
	// 	<-time.After(time.Second * 20)
	// 	finishMain <- true
	// }()

	/*
		if os.Getenv("PP_AGENT_SERVICE_TEST_MOCK") != "" {
			s.logger.Info("PP_AGENT_SERVICE_TEST_MOCK passed, running test mock export")
			err := s.runTestMockExport()
			if err != nil {
				return fmt.Errorf("could not export mock, err: %v", err)
			}
		}*/

	s.logger.Info("waiting for requests...")

	defer func() {
		s.logger.Info("Will exit in 5s, closing up...")
		go func() {
			for _, cl := range closers {
				cl()
			}
		}()
		time.Sleep(5 * time.Second)
	}()

	<-finishMain

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

	err := aevent.Publish(ctx, publishEvent, s.conf.Channel, s.conf.APIKey)
	if err != nil {
		return err
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
	return action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   topic,
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

	var failedPings []time.Time
	for {
		select {
		case <-time.After(30 * time.Second):
			err := s.sendPing(ctx)
			if err != nil {
				s.logger.Error("could not send ping", "err", err.Error())
				var pastHour []time.Time
				cutoff := time.Now().Add(-time.Hour)
				for _, p := range failedPings {
					if p.After(cutoff) {
						pastHour = append(pastHour, p)
					}
				}
				pastHour = append(pastHour, time.Now())
				failedPings = pastHour
				if len(failedPings) > 5 {
					panic("more than 5 pings failed in the past hour, exiting to restart")
				}
			} else {
				s.logger.Info("sent ping")
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
	return aevent.Publish(ctx, e, s.conf.Channel, s.conf.APIKey)
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
