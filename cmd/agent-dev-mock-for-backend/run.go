package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	"github.com/pinpt/integration-sdk/agent"
)

func handleIntegrationEvents(ctx context.Context, log hclog.Logger, apiKey string, customerID string, agentOpts agentOpts) error {
	errors := make(chan error, 1)

	actionConfig := action.Config{
		APIKey:  apiKey + "xxx",
		GroupID: fmt.Sprintf("agent-%v", agentOpts.DeviceID),
		Channel: agentOpts.Channel,
		Factory: factory,
		Topic:   agent.IntegrationRequestTopic.String(),
		Errors:  errors,
		Headers: map[string]string{
			"customer_id": customerID,
			"uuid":        agentOpts.DeviceID,
		},
	}

	done := make(chan bool)

	cb := func(instance datamodel.Model) (datamodel.Model, error) {
		defer func() { done <- true }()
		resp := instance.(*agent.IntegrationRequest)

		log.Info("received integration request", "data", resp.ToMap())

		return nil, nil
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
