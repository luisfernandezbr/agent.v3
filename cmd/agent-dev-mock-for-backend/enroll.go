package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pinpt/agent.next/cmd/cmdvalidate"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/go-common/event/action"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/datamodel"
)

func enrollRequest(ctx context.Context, log hclog.Logger, code string, agentOpts agentOpts) error {

	errors := make(chan error, 1)

	enrollConfig := action.Config{
		GroupID: fmt.Sprintf("agent-%v", agentOpts.DeviceID),
		Channel: agentOpts.Channel,
		Factory: factory,
		Topic:   agent.EnrollResponseTopic.String(),
		Errors:  errors,
		Headers: map[string]string{
			"uuid": agentOpts.DeviceID,
		},
	}

	done := make(chan bool)

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		defer func() { done <- true }()
		resp := instance.Object().(*agent.EnrollResponse)

		//log.Info("received enroll response", "data", resp.ToMap())

		valid, err := cmdvalidate.Run(ctx, log, false)
		if err != nil {
			return nil, err
		}
		if !valid {
			// return a msg here
			log.Info("the mininum requeriments were not meet")
			return nil, nil
		}

		log.Info("received agent auth data run the following")
		log.Info("run '" + resp.Apikey + "' '" + resp.CustomerID + "'")

		return nil, nil
	}

	log.Info("registering enroll")

	sub, err := action.Register(ctx, action.NewAction(cb), enrollConfig)
	if err != nil {
		panic(err)
	}

	reqData := agent.EnrollRequest{
		Code: code,
		UUID: agentOpts.DeviceID,
	}

	reqEvent := event.PublishEvent{
		Object: &reqData,
		Headers: map[string]string{
			"uuid": agentOpts.DeviceID,
		},
	}

	err = event.Publish(ctx, reqEvent, agentOpts.Channel, "")
	if err != nil {
		panic(err)
	}

	<-done
	// TODO: event subs do not close down properly
	time.Sleep(1 * time.Second)
	sub.Close()

	return nil
}
