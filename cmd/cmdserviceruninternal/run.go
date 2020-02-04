package cmdserviceruninternal

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrun"
	"github.com/pinpt/agent/cmd/pkg/ppservice"
	"github.com/pinpt/agent/pkg/service"
)

func Run(logger hclog.Logger, pinpointRoot string) error {
	service.Run(ppservice.Names, service.Opts{
		PinpointRoot: pinpointRoot,
	}, func(cancel chan bool) error {
		opts := cmdrun.Opts{
			Logger:       logger,
			PinpointRoot: pinpointRoot,
		}
		return cmdrun.Run(context.Background(), opts, cancel)
	})
	return nil
}
