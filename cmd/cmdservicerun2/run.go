package cmdservicerun2

import (
	"context"
	"fmt"

	pjson "github.com/pinpt/go-common/json"

	"github.com/pinpt/agent.next/pkg/keychain"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/agentconf2"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent.next/pkg/deviceinfo"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	"github.com/pinpt/go-common/hash"
	pstrings "github.com/pinpt/go-common/strings"
	isdk "github.com/pinpt/integration-sdk"
)

type Opts struct {
	Logger       hclog.Logger
	PinpointRoot string
	Encryptor    *keychain.Encryptor
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
	conf     agentconf2.Config
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
	s.conf, err = agentconf2.Load(s.fsconf.Config2)
	if err != nil {
		return err
	}

	s.exporter = newExporter(exporterOpts{
		Logger:       s.logger,
		CustomerID:   s.conf.CustomerID,
		PinpointRoot: s.opts.PinpointRoot,
		Encryptor:    s.opts.Encryptor,
		FSConf:       s.fsconf,
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

	errors := make(chan error, 1)

	actionConfig := action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   agent.IntegrationRequestTopic.String(),
		Errors:  errors,
		Headers: map[string]string{
			"customer_id": s.conf.CustomerID,
			//"uuid":        agentOpts.DeviceID, //NOTE: eventmachine does not send uuid
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

		if integration.Name == "jira" || integration.Name == "github" {
			authData := pjson.Stringify(integration.Authorization.ToMap())
			encrAuthData, err := s.opts.Encryptor.Encrypt(authData)
			if err != nil {
				panic(err)
			}

			resp := &agent.IntegrationResponse{}
			resp.RefType = integration.Name
			resp.RefID = hash.Values(s.conf.DeviceID, integration.Name)
			resp.Success = true
			resp.Type = agent.IntegrationResponseTypeIntegration
			resp.Authorization = encrAuthData

			return datamodel.NewModelSendEvent(resp), nil
		}

		// error for everything else
		resp := &agent.IntegrationResponse{}
		resp.RefType = integration.Name
		resp.RefID = hash.Values(s.conf.DeviceID, integration.Name)
		resp.Type = agent.IntegrationResponseTypeIntegration
		resp.Error = pstrings.Pointer("Only jira and github integrations are supported")

		return datamodel.NewModelSendEvent(resp), nil
	}

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

		s.exporter.ExportQueue <- req

		err := <-done

		jobID := ev.JobID

		data := agent.ExportResponse{
			CustomerID: s.conf.CustomerID,
			UUID:       s.conf.DeviceID,
			JobID:      jobID,
		}
		if err != nil {
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
