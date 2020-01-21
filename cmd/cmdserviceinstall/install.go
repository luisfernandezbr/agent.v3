package cmdserviceinstall

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdservicestart"
	"github.com/pinpt/agent/cmd/pkg/ppservice"
	"github.com/pinpt/agent/pkg/service"
)

func Run(logger hclog.Logger, pinpointRoot string, start bool) error {
	err := service.Install(logger, ppservice.Names, []string{
		"service-run", "--pinpoint-root", pinpointRoot,
	})
	if err != nil {
		return err
	}
	if start {
		return cmdservicestart.Run(logger)
	}
	return nil
}
