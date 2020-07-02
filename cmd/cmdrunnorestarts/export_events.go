package cmdrunnorestarts

import (
	"context"
	"fmt"

	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/go-common/event/action"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/exporter"
)

func (s *runner) handleExportEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for export requests")

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
		Topic:   agent.ExportRequestModelName.String(),
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {

		ev := instance.Object().(*agent.ExportRequest)
		s.logger.Info("received export request", "id", ev.ID, "uuid", ev.UUID, "request_date", ev.RequestDate.Rfc3339)

		header, err := parseHeader(instance.Message().Headers)
		if err != nil {
			return nil, fmt.Errorf("error parsing header. err %v", err)
		}
		s.exporter.ExportQueue <- exporter.Request{
			Data:      ev,
			MessageID: header.MessageID,
		}
		return nil, nil
	}

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}

	sub.WaitForReady()

	return func() { sub.Close() }, nil

}
