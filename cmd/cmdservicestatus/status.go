package cmdservicestatus

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrun"
	"github.com/pinpt/agent/pkg/service"
)

func Run(logger hclog.Logger) error {
	return service.Run(service.Status, nil, cmdrun.Opts{
		Logger: logger,
	})
}
