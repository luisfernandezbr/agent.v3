package cmdservicerunnorestarts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/build"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/structmarshal"

	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/agent.next/cmd/cmdexportonboarddata"
	"github.com/pinpt/agent.next/cmd/cmdintegration"

	"github.com/pinpt/agent.next/pkg/encrypt"

	"github.com/pinpt/go-common/eventing"
	"github.com/pinpt/go-common/hash"
	pjson "github.com/pinpt/go-common/json"

	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent.next/cmd/pkg/cmdlogger"
	"github.com/pinpt/agent.next/pkg/deviceinfo"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	pstrings "github.com/pinpt/go-common/strings"
	isdk "github.com/pinpt/integration-sdk"

	"github.com/pinpt/agent.next/cmd/cmdservicerunnorestarts/exporter"
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

	logSender *logSender

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

	s.logSender = newLogSender(s.opts.Logger, s.conf, "service-run", hash.Values(time.Now()))
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
			err := build.ValidateVersion(version)
			if err != nil {
				return fmt.Errorf("Could not self-update, invalid version in PP_AGENT_UPDATE_VERSION: %v", err)
			}
			if version == os.Getenv("PP_AGENT_VERSION") {
				s.logger.Info("already at wanted version", "v", version)
			} else {
				err = s.update(version)
				if err != nil {
					return fmt.Errorf("Could not self-update: %v", err)
				}
				s.logger.Info("Updated the agent, restarting...")
				return nil
			}
		} else {
			err := s.downloadIntegrationsIfMissing()
			if err != nil {
				return fmt.Errorf("Could not download integration binaries: %v", err)
			}
		}
	}

	err := s.sendCrashes()
	if err != nil {
		return fmt.Errorf("could not send crashes, err: %v", err)
	}

	err = s.sendStart(context.Background())
	if err != nil {
		return fmt.Errorf("could not send start event, err: %v", err)
	}

	s.exporter = newExporter(exporterOpts{
		Logger:              s.logger,
		LogLevelSubcommands: s.opts.LogLevelSubcommands,
		PinpointRoot:        s.opts.PinpointRoot,
		Conf:                s.conf,
		FSConf:              s.fsconf,
		PPEncryptionKey:     s.conf.PPEncryptionKey,
		AgentConfig:         s.agentConfig,
		IntegrationsDir:     s.fsconf.Integrations,
	})

	go func() {
		s.sendPings()
	}()
	go func() {
		s.exporter.Run()
	}()

	err = s.sendEnabled(ctx)
	if err != nil {
		return fmt.Errorf("could not send enabled request, err: %v", err)
	}

	isub, err := s.handleIntegrationEvents(ctx)
	if err != nil {
		return fmt.Errorf("error handling integration requests, err: %v", err)
	}
	defer isub()

	osub, err := s.handleOnboardingEvents(ctx)
	if err != nil {
		return fmt.Errorf("error handling onboarding requests, err: %v", err)
	}

	defer osub()

	esub, err := s.handleExportEvents(ctx)
	if err != nil {
		return fmt.Errorf("error handling export requests, err: %v", err)
	}

	defer esub()

	if os.Getenv("PP_AGENT_SERVICE_TEST_MOCK") != "" {
		s.logger.Info("PP_AGENT_SERVICE_TEST_MOCK passed, running test mock export")
		err := s.runTestMockExport()
		if err != nil {
			return fmt.Errorf("could not export mock, err: %v", err)
		}
	}

	s.logger.Info("waiting for requests...")

	block := make(chan bool)
	<-block

	return nil
}

func (s *runner) runTestMockExport() error {

	in := cmdexport.Integration{}
	in.Name = "mock"
	in.Config = map[string]interface{}{"k1": "v1"}
	integrations := []cmdexport.Integration{in}
	reprocessHistorical := true

	ctx := context.Background()

	return s.exporter.execExport(ctx, integrations, reprocessHistorical, "mock-msg-id", "mock-job-id")
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

func (s *runner) handleIntegrationEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for integration requests")

	errorsChan := make(chan error, 1)

	actionConfig := action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   agent.IntegrationRequestTopic.String(),
		Errors:  errorsChan,
		Headers: map[string]string{
			"customer_id": s.conf.CustomerID,
			"uuid":        s.conf.DeviceID,
		},
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.IntegrationRequest)
		headers, err := parseHeader(instance.Message().Headers)
		if err != nil {
			return nil, fmt.Errorf("error parsing header. err %v", err)
		}
		integration := req.Integration

		s.logger.Info("received integration request", "integration", integration.Name)

		s.logger.Info("sending back integration response")

		// TODO: add connection validation

		sendEvent := func(resp *agent.IntegrationResponse) (datamodel.ModelSendEvent, error) {
			s.deviceInfo.AppendCommonInfo(resp)
			return datamodel.NewModelSendEvent(resp), nil
		}

		resp := &agent.IntegrationResponse{}
		resp.RefType = integration.Name
		resp.RefID = integration.RefID
		resp.RequestID = req.ID

		resp.UUID = s.conf.DeviceID
		date.ConvertToModel(time.Now(), &resp.EventDate)

		rerr := func(err error) (datamodel.ModelSendEvent, error) {
			s.logger.Error("integration request failed", "err", err)
			// error for everything else
			resp.Type = agent.IntegrationResponseTypeIntegration
			resp.Error = pstrings.Pointer(err.Error())
			return sendEvent(resp)
		}

		auth := integration.Authorization.ToMap()

		res, err := s.validate(ctx, integration.Name, headers.MessageID, IntegrationType(req.Integration.SystemType), auth)
		if err != nil {
			return rerr(fmt.Errorf("could not call validate, err: %v", err))
		}

		if !res.Success {
			return rerr(errors.New(strings.Join(res.Errors, ", ")))
		}

		encrAuthData, err := encrypt.EncryptString(pjson.Stringify(auth), s.conf.PPEncryptionKey)
		if err != nil {
			return rerr(err)
		}

		resp.Message = "Success. Integration validated."
		resp.Success = true
		resp.Type = agent.IntegrationResponseTypeIntegration
		resp.Authorization = encrAuthData
		return sendEvent(resp)
	}

	go func() {
		for err := range errorsChan {
			s.logger.Error("error in integration requests", "err", err)
		}
	}()

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}

	sub.WaitForReady()

	return func() { sub.Close() }, nil
}

func (s *runner) handleOnboardingEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for onboarding requests")

	processOnboard := func(msg eventing.Message, integration map[string]interface{}, systemType IntegrationType, objectType string) (data cmdexportonboarddata.Result, rerr error) {
		atomic.AddInt64(&s.onboardingInProgress, 1)
		defer func() {
			atomic.AddInt64(&s.onboardingInProgress, -1)
		}()

		s.logger.Info("received onboard request", "type", objectType)
		header, err := parseHeader(msg.Headers)
		if err != nil {
			return data, fmt.Errorf("error parsing header. err %v", err)
		}

		ctx := context.Background()
		conf, err := configFromEvent(integration, systemType, s.conf.PPEncryptionKey)
		if err != nil {
			rerr = err
			return
		}

		data, err = s.getOnboardData(ctx, conf, header.MessageID, objectType)
		if err != nil {
			rerr = err
			return
		}

		return data, nil
	}

	cbUser := func(instance datamodel.ModelReceiveEvent) (_ datamodel.ModelSendEvent, _ error) {

		rerr := func(err error) {
			s.logger.Error("could not process onboard event", "err", err)
		}

		req := instance.Object().(*agent.UserRequest)
		data, err := processOnboard(instance.Message(), req.Integration.ToMap(), IntegrationType(req.Integration.SystemType), "users")
		if err != nil {
			rerr(err)
			return
		}
		resp := &agent.UserResponse{}
		resp.Type = agent.UserResponseTypeUser
		resp.RefType = req.RefType
		resp.RefID = req.RefID
		resp.RequestID = req.ID
		resp.IntegrationID = req.Integration.ID

		resp.Success = data.Success
		if data.Error != "" {
			resp.Error = pstrings.Pointer(data.Error)
		}
		if data.Data != nil {
			var obj cmdexportonboarddata.DataUsers
			err := structmarshal.AnyToAny(data.Data, &obj)
			if err != nil {
				rerr(fmt.Errorf("invalid data format returned in agent onboard: %v", err))
			}
			for _, rec := range obj.Users {
				user := agent.UserResponseUsers{}
				user.FromMap(rec)
				resp.Users = append(resp.Users, user)
			}
			for _, rec := range obj.Teams {
				team := agent.UserResponseTeams{}
				team.FromMap(rec)
				resp.Teams = append(resp.Teams, team)
			}
		}
		s.deviceInfo.AppendCommonInfo(resp)
		return datamodel.NewModelSendEvent(resp), nil
	}

	cbRepo := func(instance datamodel.ModelReceiveEvent) (_ datamodel.ModelSendEvent, _ error) {

		rerr := func(err error) {
			s.logger.Error("could not process onboard event", "err", err)
		}

		req := instance.Object().(*agent.RepoRequest)
		data, err := processOnboard(instance.Message(), req.Integration.ToMap(), IntegrationType(req.Integration.SystemType), "repos")

		if err != nil {
			rerr(err)
			return
		}

		resp := &agent.RepoResponse{}
		resp.Type = agent.RepoResponseTypeRepo
		resp.RefType = req.RefType
		resp.RefID = req.RefID
		resp.RequestID = req.ID
		resp.IntegrationID = req.Integration.ID

		resp.Success = data.Success
		if data.Error != "" {
			resp.Error = pstrings.Pointer(data.Error)
		}

		if data.Data != nil {
			var records cmdexportonboarddata.DataRepos
			err := structmarshal.AnyToAny(data.Data, &records)
			if err != nil {
				rerr(fmt.Errorf("invalid data format returned in agent onboard: %v", err))
			}
			for _, rec := range records {
				repo := &agent.RepoResponseRepos{}
				repo.FromMap(rec)
				resp.Repos = append(resp.Repos, *repo)
			}
		}

		s.deviceInfo.AppendCommonInfo(resp)
		return datamodel.NewModelSendEvent(resp), nil
	}

	cbProject := func(instance datamodel.ModelReceiveEvent) (_ datamodel.ModelSendEvent, _ error) {

		rerr := func(err error) {
			s.logger.Error("could not process onboard event", "err", err)
		}

		req := instance.Object().(*agent.ProjectRequest)
		data, err := processOnboard(instance.Message(), req.Integration.ToMap(), IntegrationType(req.Integration.SystemType), "projects")
		if err != nil {
			rerr(err)
			return
		}

		resp := &agent.ProjectResponse{}
		resp.Type = agent.ProjectResponseTypeProject
		resp.RefType = req.RefType
		resp.RefID = req.RefID
		resp.RequestID = req.ID
		resp.IntegrationID = req.Integration.ID

		resp.Success = data.Success
		if data.Error != "" {
			resp.Error = pstrings.Pointer(data.Error)
		}

		if data.Data != nil {
			var records cmdexportonboarddata.DataProjects
			err := structmarshal.AnyToAny(data.Data, &records)
			if err != nil {
				rerr(fmt.Errorf("invalid data format returned in agent onboard: %v", err))
			}
			for _, rec := range records {
				project := &agent.ProjectResponseProjects{}
				project.FromMap(rec)
				resp.Projects = append(resp.Projects, *project)
			}
		}
		s.deviceInfo.AppendCommonInfo(resp)
		return datamodel.NewModelSendEvent(resp), nil
	}

	cbWorkconfig := func(instance datamodel.ModelReceiveEvent) (_ datamodel.ModelSendEvent, _ error) {

		rerr := func(err error) {
			s.logger.Error("could not process onboard event", "err", err)
		}

		req := instance.Object().(*agent.WorkStatusRequest)
		data, err := processOnboard(instance.Message(), req.Integration.ToMap(), IntegrationType(req.Integration.SystemType), "workconfig")
		if err != nil {
			rerr(err)
			return
		}

		resp := &agent.WorkStatusResponse{}
		resp.Type = agent.WorkStatusResponseTypeProject
		resp.RefType = req.RefType
		resp.RefID = req.RefID
		resp.RequestID = req.ID
		resp.IntegrationID = req.Integration.ID

		resp.Success = data.Success
		if data.Error != "" {
			resp.Error = pstrings.Pointer(data.Error)
		}

		workStatuses := &agent.WorkStatusResponseWorkConfig{}
		if data.Data != nil {
			var m cmdexportonboarddata.DataWorkConfig
			err := structmarshal.AnyToAny(data.Data, &m)
			if err != nil {
				rerr(fmt.Errorf("invalid data format returned in agent onboard: %v", err))
			}
			workStatuses.FromMap(m)
			resp.WorkConfig = *workStatuses
		}

		s.deviceInfo.AppendCommonInfo(resp)

		return datamodel.NewModelSendEvent(resp), nil
	}

	usub, err := action.Register(ctx, action.NewAction(cbUser), s.newSubConfig(agent.UserRequestTopic.String()))
	if err != nil {
		return nil, err
	}

	rsub, err := action.Register(ctx, action.NewAction(cbRepo), s.newSubConfig(agent.RepoRequestTopic.String()))
	if err != nil {
		return nil, err
	}

	psub, err := action.Register(ctx, action.NewAction(cbProject), s.newSubConfig(agent.ProjectRequestTopic.String()))
	if err != nil {
		return nil, err
	}

	wsub, err := action.Register(ctx, action.NewAction(cbWorkconfig), s.newSubConfig(agent.WorkStatusRequestTopic.String()))
	if err != nil {
		panic(err)
	}

	usub.WaitForReady()
	rsub.WaitForReady()
	psub.WaitForReady()
	wsub.WaitForReady()

	return func() {
		usub.Close()
		rsub.Close()
		psub.Close()
		wsub.Close()
	}, nil
}

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

func (s *runner) handleExportEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for export requests")

	errors := make(chan error, 1)

	actionConfig := action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   agent.ExportRequestTopic.String(),
		Errors:  errors,
		Headers: map[string]string{
			"customer_id": s.conf.CustomerID,
			"uuid":        s.conf.DeviceID,
		},
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {

		ev := instance.Object().(*agent.ExportRequest)
		s.logger.Info("received export request", "id", ev.ID, "uuid", ev.UUID, "request_date", ev.RequestDate.Rfc3339)
		header, err := parseHeader(instance.Message().Headers)
		if err != nil {
			return nil, fmt.Errorf("error parsing header. err %v", err)
		}
		done := make(chan bool)
		s.exporter.ExportQueue <- exportRequest{
			Done:      done,
			Data:      ev,
			MessageID: header.MessageID,
		}
		<-done
		return nil, nil
	}

	go func() {
		for err := range errors {
			s.logger.Error("error in integration requests", "err", err)
		}
	}()

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}

	sub.WaitForReady()

	return func() { sub.Close() }, nil

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
	return s.sendEvent(ctx, ev, "", nil)
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
