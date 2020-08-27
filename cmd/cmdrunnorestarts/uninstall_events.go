package cmdrunnorestarts

import (
	"context"
	"fmt"

	"github.com/pinpt/go-common/v10/datamodel"
	"github.com/pinpt/go-common/v10/event"
	"github.com/pinpt/go-common/v10/event/action"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *runner) handleUninstallEvents(ctx context.Context, finishMain chan bool) (closefunc, error) {
	s.logger.Info("listening for uninstall requests")

	actionConfig := action.Config{
		Subscription: event.Subscription{
			APIKey:  s.conf.APIKey,
			GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
			Channel: s.conf.Channel,
			Headers: map[string]string{
				"customer_id": s.conf.CustomerID,
				"uuid":        s.conf.DeviceID,
			},
			DisablePing: true,
		},
		Factory: factory,
		Topic:   agent.UninstallRequestModelName.String(),
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {

		s.logger.Info("received uninstall request")

		defer func() { finishMain <- true }()
		return nil, nil
	}

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}

	sub.WaitForReady()

	return func() { sub.Close() }, nil
}
