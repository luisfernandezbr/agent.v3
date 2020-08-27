package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/v10/event"
	"github.com/pinpt/go-common/v10/event/action"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/v10/datamodel"
)

func enrollRequest(ctx context.Context, log hclog.Logger, code string, agentOpts agentOpts) error {

	errors := make(chan error, 1)

	enrollConfig := action.Config{
		Subscription: event.Subscription{
			GroupID: fmt.Sprintf("agent-%v", agentOpts.DeviceID),
			Channel: agentOpts.Channel,
			Errors:  errors,
			Headers: map[string]string{
				"uuid": agentOpts.DeviceID,
			},
			DisablePing: true,
		},

		Factory: factory,
		Topic:   agent.EnrollResponseModelName.String(),
	}

	done := make(chan bool)

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		defer func() { done <- true }()
		resp := instance.Object().(*agent.EnrollResponse)

		//log.Info("received enroll response", "data", resp.ToMap())

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
