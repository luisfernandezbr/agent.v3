package cmdservicerunnorestarts

import (
	"context"
	"fmt"
	"time"

	"github.com/pinpt/agent/cmd/cmdservicerunnorestarts/subcommand"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *runner) handleCancelEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for cancel requests")

	errors := make(chan error, 1)
	actionConfig := action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   agent.CancelRequestTopic.String(),
		Errors:  errors,
		Headers: map[string]string{
			"customer_id": s.conf.CustomerID,
			"uuid":        s.conf.DeviceID,
		},
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		ev := instance.Object().(*agent.CancelRequest)

		var cmdname string
		switch ev.Command {
		case agent.CancelRequestCommandEXPORT:
			cmdname = "export"
		case agent.CancelRequestCommandONBOARD:
			cmdname = "export-onboard-data"
		case agent.CancelRequestCommandINTEGRATION:
			cmdname = "validate-config"
		}
		resp := &agent.CancelResponse{}
		resp.Success = true
		s.deviceInfo.AppendCommonInfo(resp)
		date.ConvertToModel(time.Now(), &resp.CancelDate)

		if cmdname == "" {
			err := fmt.Errorf("wrong command %s", ev.Command.String())
			errstr := err.Error()
			resp.Error = &errstr
			s.logger.Error("error in cancel request", "err", err)

		} else {
			if err := subcommand.KillCommand(s.logger, cmdname); err != nil {
				errstr := err.Error()
				resp.Error = &errstr
				s.logger.Error("error processing cancel request", "err", err.Error())
			}
		}
		return datamodel.NewModelSendEvent(resp), nil
	}

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}
	go func() {
		for err := range errors {
			s.logger.Error("error in integration requests", "err", err)
		}
	}()

	sub.WaitForReady()

	return func() { sub.Close() }, nil
}
