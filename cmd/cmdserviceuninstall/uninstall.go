package cmdserviceuninstall

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	kservice "github.com/kardianos/service"
	"github.com/pinpt/agent/cmd/cmdserviceinstall"
)

func Run(logger hclog.Logger) error {
	conf := cmdserviceinstall.KServiceConfig()
	serv, err := kservice.New(nil, conf)
	if err != nil {
		return fmt.Errorf("could not init service, err: %v", err)
	}
	err = serv.Stop()
	if err != nil {
		return fmt.Errorf("error stopping service, err: %v", err)
	}

	return nil
}
