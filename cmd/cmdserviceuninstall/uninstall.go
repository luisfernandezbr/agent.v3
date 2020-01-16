package cmdserviceuninstall

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/pkg/ppservice"
	"github.com/pinpt/agent/pkg/service"
)

func Run(logger hclog.Logger) error {
	return service.Control(logger, ppservice.Names, service.Uninstall)
}
