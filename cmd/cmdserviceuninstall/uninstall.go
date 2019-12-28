package cmdserviceuninstall

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrun"
	"github.com/pinpt/agent/pkg/service"
)

func Run(logger hclog.Logger) error {
	return service.Run(service.Uninstall, nil, cmdrun.Opts{
		Logger: logger,
	})
}
