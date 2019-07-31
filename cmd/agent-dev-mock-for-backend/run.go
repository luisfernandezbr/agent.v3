package main

import (
	"context"
	"fmt"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/go-common/hash"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	"github.com/pinpt/integration-sdk/agent"
)

func handleIntegrationEvents(ctx context.Context, log hclog.Logger, apiKey string, customerID string, agentOpts agentOpts) error {
	errors := make(chan error, 1)

	actionConfig := action.Config{
		APIKey:  apiKey,
		GroupID: fmt.Sprintf("agent-%v", agentOpts.DeviceID),
		Channel: agentOpts.Channel,
		Factory: factory,
		Topic:   agent.IntegrationRequestTopic.String(),
		Errors:  errors,
		Headers: map[string]string{
			"customer_id": customerID,
			//"uuid":        agentOpts.DeviceID, //NOTE: eventmachine does not send uuid
		},
	}

	cb := func(instance datamodel.Model) (datamodel.Model, error) {
		req := instance.(*agent.IntegrationRequest)

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
			return resp, nil
		}

		// error for everything else
		resp := &agent.IntegrationResponse{}
		resp.RefType = integration.Name
		resp.RefID = hash.Values(agentOpts.DeviceID, integration.Name)
		resp.Type = agent.IntegrationResponseTypeIntegration
		resp.Error = pstrings.Pointer("Only jira returns successful IntegrationResponse for this mock")

		return resp, nil
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

func runService(ctx context.Context, log hclog.Logger, apiKey string, customerID string, agentOpts agentOpts) error {

	err := handleIntegrationEvents(ctx, log, apiKey, customerID, agentOpts)
	if err != nil {
		panic(err)
	}

	block := make(chan bool)
	<-block
	return nil
}
