package cmdservicerestart

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrun"
	"github.com/pinpt/agent/pkg/service"
)

func Run(logger hclog.Logger) error {
	return service.Run(service.Restart, nil, cmdrun.Opts{
		Logger: logger,
	})
}
