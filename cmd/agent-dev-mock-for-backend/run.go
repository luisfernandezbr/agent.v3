package main

import (
	"context"
	"fmt"

	"github.com/pinpt/go-common/v10/event"
	pstrings "github.com/pinpt/go-common/v10/strings"

	"github.com/pinpt/go-common/v10/hash"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/v10/datamodel"
	"github.com/pinpt/go-common/v10/event/action"
	"github.com/pinpt/integration-sdk/agent"
)

func runService(ctx context.Context, log hclog.Logger, apiKey string, customerID string, agentOpts agentOpts) error {
	log.Info("sending enabled request")

	err := sendEnabled(ctx, log, apiKey, customerID, agentOpts)
	if err != nil {
		panic(err)
	}

	err = handleIntegrationEvents(ctx, log, apiKey, customerID, agentOpts)
	if err != nil {
		panic(err)
	}

	err = handleExportEvents(ctx, log, apiKey, customerID, agentOpts)
	if err != nil {
		panic(err)
	}

	block := make(chan bool)
	<-block
	return nil
}

func sendEnabled(ctx context.Context, log hclog.Logger, apiKey string, customerID string, agentOpts agentOpts) error {

	data := agent.Enabled{
		CustomerID: customerID,
		UUID:       agentOpts.DeviceID,
	}

	publishEvent := event.PublishEvent{
		Object: &data,
		Headers: map[string]string{
			"uuid": agentOpts.DeviceID,
		},
	}

	err := event.Publish(ctx, publishEvent, agentOpts.Channel, apiKey)
	if err != nil {
		panic(err)
	}

	return nil
}

func handleIntegrationEvents(ctx context.Context, log hclog.Logger, apiKey string, customerID string, agentOpts agentOpts) error {
	errors := make(chan error, 1)

	actionConfig := action.Config{
		Subscription: event.Subscription{
			APIKey:  apiKey,
			GroupID: fmt.Sprintf("agent-%v", agentOpts.DeviceID),
			Channel: agentOpts.Channel,
			Errors:  errors,
			Headers: map[string]string{
				"customer_id": customerID,
				//"uuid":        agentOpts.DeviceID, //NOTE: eventmachine does not send uuid
			},
			DisablePing: true,
		},

		Factory: factory,
		Topic:   agent.IntegrationRequestModelName.String(),
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.IntegrationRequest)

		log.Info("received integration request", "data", req.ToMap())

		// validate the integration data here

		integration := req.Integration

		log.Info("authorization", "data", integration.Authorization.ToMap())

		log.Info("sending back integration response")

		// success for jira
		if integration.Name == "jira" {
			resp := &agent.IntegrationResponse{}
			resp.RefType = integration.Name
			resp.RefID = hash.Values(agentOpts.DeviceID, integration.Name)
			resp.Success = true
			resp.Type = agent.IntegrationResponseTypeIntegration
			resp.Authorization = "encrypted blob data"
			return datamodel.NewModelSendEvent(resp), nil
		}

		// error for everything else
		resp := &agent.IntegrationResponse{}
		resp.RefType = integration.Name
		resp.RefID = hash.Values(agentOpts.DeviceID, integration.Name)
		resp.Type = agent.IntegrationResponseTypeIntegration
		resp.Error = pstrings.Pointer("Only jira returns successful IntegrationResponse for this mock")

		return datamodel.NewModelSendEvent(resp), nil
	}

	log.Info("listening for integration request")

	go func() {
		for err := range errors {
			log.Error("error in integration events", "err", err)
		}
	}()

	_, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		panic(err)
	}

	return nil
}

func handleExportEvents(ctx context.Context, log hclog.Logger, apiKey string, customerID string, agentOpts agentOpts) error {
	errors := make(chan error, 1)

	actionConfig := action.Config{
		Subscription: event.Subscription{
			APIKey:  apiKey,
			GroupID: fmt.Sprintf("agent-%v", agentOpts.DeviceID),
			Channel: agentOpts.Channel,
			Errors:  errors,
			Headers: map[string]string{
				"customer_id": customerID,
				//"uuid":        agentOpts.DeviceID, //NOTE: eventmachine does not send uuid
			},
			DisablePing: true,
		},
		Factory: factory,
		Topic:   agent.ExportRequestModelName.String(),
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.ExportRequest)

		log.Info("received export request", "data", req.ToMap())

		jobID := req.JobID
		uploadURL := req.UploadURL

		for _, integration := range req.Integrations {

			log.Info("processing integration", "name", integration.Name, "job_id", jobID, "upload_url", uploadURL)

			data := agent.ExportResponse{
				CustomerID: customerID,
				UUID:       agentOpts.DeviceID,
				JobID:      jobID,
			}

			publishEvent := event.PublishEvent{
				Object: &data,
				Headers: map[string]string{
					"uuid": agentOpts.DeviceID,
				},
			}

			err := event.Publish(ctx, publishEvent, agentOpts.Channel, apiKey)
			if err != nil {
				panic(err)
			}

			log.Info("sent back export result")

		}

		return nil, nil
	}

	log.Info("listening for export requests")
	go func() {
		for err := range errors {
			log.Error("error in integration events", "err", err)
		}
	}()

	_, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		panic(err)
	}

	return nil
}
