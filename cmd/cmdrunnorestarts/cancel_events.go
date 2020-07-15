package cmdrunnorestarts

import (
	"context"
	"fmt"
	"time"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/subcommand"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/go-common/v10/datamodel"
	"github.com/pinpt/go-common/v10/event"
	"github.com/pinpt/go-common/v10/event/action"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *runner) handleCancelEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for cancel requests")

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
		Topic:   agent.CancelRequestModelName.String(),
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		ev := instance.Object().(*agent.CancelRequest)

		var cmdname string
		switch ev.Command {
		case agent.CancelRequestCommandExport:
			cmdname = "export"
		case agent.CancelRequestCommandOnboard:
			cmdname = "export-onboard-data"
		case agent.CancelRequestCommandIntegration:
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
			if err := subcommand.KillCommand(subcommand.KillCmdOpts{
				PrintLog: func(msg string, args ...interface{}) {
					s.logger.Debug(msg, args)
				},
			}, cmdname); err != nil {
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

	sub.WaitForReady()

	return func() { sub.Close() }, nil
}
