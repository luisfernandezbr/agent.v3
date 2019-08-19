package cmdservicerun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/date"

	"github.com/pinpt/agent.next/cmd/cmdexportonboarddata"

	"github.com/pinpt/agent.next/pkg/encrypt"

	pjson "github.com/pinpt/go-common/json"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent.next/pkg/deviceinfo"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	pstrings "github.com/pinpt/go-common/strings"
	isdk "github.com/pinpt/integration-sdk"
)

type Opts struct {
	Logger       hclog.Logger
	PinpointRoot string
}

func Run(ctx context.Context, opts Opts) error {
	run, err := newRunner(opts)
	if err != nil {
		return err
	}
	return run.run(ctx)
}

type runner struct {
	opts     Opts
	logger   hclog.Logger
	fsconf   fsconf.Locs
	conf     agentconf.Config
	exporter *exporter
}

func newRunner(opts Opts) (*runner, error) {
	s := &runner{}
	s.opts = opts
	s.logger = opts.Logger
	s.fsconf = fsconf.New(opts.PinpointRoot)
	return s, nil
}

func (s *runner) run(ctx context.Context) error {
	s.logger.Info("starting service")

	var err error
	s.conf, err = agentconf.Load(s.fsconf.Config2)
	if err != nil {
		return err
	}

	go func() {
		s.sendPings()
	}()

	err = s.sendStart(context.Background())
	if err != nil {
		return err
	}

	s.exporter = newExporter(exporterOpts{
		Logger:          s.logger,
		CustomerID:      s.conf.CustomerID,
		PinpointRoot:    s.opts.PinpointRoot,
		FSConf:          s.fsconf,
		PPEncryptionKey: s.conf.PPEncryptionKey,
	})

	go func() {
		s.exporter.Run()
	}()

	err = s.sendEnabled(ctx)
	if err != nil {
		return err
	}

	err = s.handleIntegrationEvents(ctx)
	if err != nil {
		return err
	}

	err = s.handleOnboardingEvents(ctx)
	if err != nil {
		return err
	}

	err = s.handleExportEvents(ctx)
	if err != nil {
		return err
	}

	s.logger.Info("waiting for events...")

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
	deviceinfo.AppendCommonInfo(&data, s.conf.CustomerID)

	publishEvent := event.PublishEvent{
		Object: &data,
		Headers: map[string]string{
			"uuid": s.conf.DeviceID,
		},
	}

	err := event.Publish(ctx, publishEvent, s.conf.Channel, s.conf.Channel)
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

func (s *runner) handleIntegrationEvents(ctx context.Context) error {
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

		integration := req.Integration

		s.logger.Info("received integration request", "integration", integration.Name)

		//s.logger.Info("received integration request", "data", req.ToMap())

		// validate the integration data here

		//s.logger.Info("authorization", "data", integration.Authorization.ToMap())

		s.logger.Info("sending back integration response")

		// TODO: add connection validation

		sendEvent := func(resp *agent.IntegrationResponse) (datamodel.ModelSendEvent, error) {
			deviceinfo.AppendCommonInfo(resp, s.conf.CustomerID)
			return datamodel.NewModelSendEvent(resp), nil
		}

		resp := &agent.IntegrationResponse{}
		resp.RefType = integration.Name
		resp.RefID = integration.RefID
		resp.RequestID = req.ID

		resp.UUID = s.conf.DeviceID
		date.ConvertToModel(time.Now(), &resp.EventDate)
		//resp.RefID = hash.Values(s.conf.DeviceID, integration.Name)

		rerr := func(err error) (datamodel.ModelSendEvent, error) {
			// error for everything else
			resp.Type = agent.IntegrationResponseTypeIntegration
			resp.Error = pstrings.Pointer(err.Error())
			return sendEvent(resp)
		}

		if integration.Name == "jira" || integration.Name == "github" {
			auth := integration.Authorization.ToMap()

			res, err := s.validate(ctx, integration.Name, auth)
			if err != nil {
				return rerr(err)
			}

			if !res.Success {
				return rerr(errors.New(strings.Join(res.Errors, ", ")))
			}

			encrAuthData, err := encrypt.EncryptString(pjson.Stringify(auth), s.conf.PPEncryptionKey)
			if err != nil {
				return rerr(err)
			}

			resp.Message = "Success. Export completed."
			resp.Success = true
			resp.Type = agent.IntegrationResponseTypeIntegration
			resp.Authorization = encrAuthData
			return sendEvent(resp)
		}

		return rerr(errors.New("Only jira and github integrations are supported"))
	}

	go func() {
		for err := range errorsChan {
			s.logger.Error("error in integration events", "err", err)
		}
	}()

	_, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		panic(err)
	}

	return nil

}

func (s *runner) handleOnboardingEvents(ctx context.Context) error {
	s.logger.Info("listening for onboarding events")

	processOnboard := func(integration map[string]interface{}, objectType string) (cmdexportonboarddata.Result, error) {
		s.logger.Info("received onboard request", "type", objectType)

		ctx := context.Background()
		conf, err := configFromEvent(integration, s.conf.PPEncryptionKey)
		if err != nil {
			panic(err)
		}

		data, err := s.getOnboardData(ctx, conf, objectType)
		if err != nil {
			panic(err)
		}

		return data, nil
	}

	cbUser := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.UserRequest)
		data, err := processOnboard(req.Integration.ToMap(), "users")
		if err != nil {
			panic(err)
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
		for _, rec := range data.Records {
			user := &agent.UserResponseUsers{}
			user.FromMap(rec)
			resp.Users = append(resp.Users, *user)
		}
		deviceinfo.AppendCommonInfo(resp, s.conf.CustomerID)
		return datamodel.NewModelSendEvent(resp), nil
	}

	cbRepo := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.RepoRequest)
		data, err := processOnboard(req.Integration.ToMap(), "repos")
		if err != nil {
			panic(err)
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

		for _, rec := range data.Records {
			repo := &agent.RepoResponseRepos{}
			repo.FromMap(rec)
			resp.Repos = append(resp.Repos, *repo)
		}
		deviceinfo.AppendCommonInfo(resp, s.conf.CustomerID)
		return datamodel.NewModelSendEvent(resp), nil
	}

	cbProject := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.ProjectRequest)
		data, err := processOnboard(req.Integration.ToMap(), "projects")
		if err != nil {
			panic(err)
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
		for _, rec := range data.Records {
			project := &agent.ProjectResponseProjects{}
			project.FromMap(rec)
			resp.Projects = append(resp.Projects, *project)
		}
		deviceinfo.AppendCommonInfo(resp, s.conf.CustomerID)
		return datamodel.NewModelSendEvent(resp), nil
	}

	_, err := action.Register(ctx, action.NewAction(cbUser), s.newSubConfig(agent.UserRequestTopic.String()))
	if err != nil {
		panic(err)
	}

	_, err = action.Register(ctx, action.NewAction(cbRepo), s.newSubConfig(agent.RepoRequestTopic.String()))
	if err != nil {
		panic(err)
	}

	_, err = action.Register(ctx, action.NewAction(cbProject), s.newSubConfig(agent.ProjectRequestTopic.String()))
	if err != nil {
		panic(err)
	}

	return nil
}

func (s *runner) newSubConfig(topic string) action.Config {
	errorsChan := make(chan error, 1)
	go func() {
		for err := range errorsChan {
			s.logger.Error("error in integration events", "err", err)
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
	}
}

func (s *runner) handleExportEvents(ctx context.Context) error {
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
			//"uuid":        agentOpts.DeviceID, //NOTE: eventmachine does not send uuid
		},
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		s.logger.Info("received export request")
		ev := instance.Object().(*agent.ExportRequest)

		done := make(chan error)

		req := exportRequest{
			Done: done,
			Data: ev,
		}

		startDate := time.Now()

		s.exporter.ExportQueue <- req

		err := <-done

		jobID := ev.JobID

		data := agent.ExportResponse{
			CustomerID: s.conf.CustomerID,
			UUID:       s.conf.DeviceID,
			JobID:      jobID,
		}
		date.ConvertToModel(startDate, &data.StartDate)

		deviceinfo.AppendCommonInfo(&data, s.conf.CustomerID)

		if err == nil {
			data.Success = true
		} else {
			data.Error = pstrings.Pointer(err.Error())
		}

		publishEvent := event.PublishEvent{
			Object: &data,
			Headers: map[string]string{
				"uuid": s.conf.DeviceID,
			},
		}

		err = event.Publish(ctx, publishEvent, s.conf.Channel, s.conf.APIKey)
		if err != nil {
			panic(err)
		}

		s.logger.Info("sent back export result")

		return nil, nil
	}

	s.logger.Info("listening for export requests")
	go func() {
		for err := range errors {
			s.logger.Error("error in integration events", "err", err)
		}
	}()

	_, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		panic(err)
	}

	return nil

}

func (s *runner) sendPings() {
	for {
		select {
		case <-time.After(10 * time.Second):
			ctx := context.Background()
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
	agentEvent := &agent.Ping{
		Type:    agent.PingTypePing,
		Success: true,
	}
	return s.sendEvent(ctx, agentEvent, "", nil)
}

func (s *runner) sendEvent(ctx context.Context, agentEvent datamodel.Model, jobID string, extraHeaders map[string]string) error {
	deviceinfo.AppendCommonInfo(agentEvent, s.conf.CustomerID)
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
