package cmdserviceuninstall

import (
	kservice "github.com/kardianos/service"
	"github.com/pinpt/agent2/cmd/cmdserviceinstall"
)

func Run() {
	conf := cmdserviceinstall.KServiceConfig()
	serv, err := kservice.New(nil, conf)
	if err != nil {
		panic(err)
		//log.Fatal(logger, "service config", "err", err)
	}
	err = serv.Stop()
	if err != nil {
		panic(err)
		//log.Warn(logger, "error stopping service", "err", err)
	}

}
