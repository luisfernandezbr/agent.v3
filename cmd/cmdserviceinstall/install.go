package cmdserviceinstall

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/kardianos/service"
	kservice "github.com/kardianos/service"
)

func Run(logger hclog.Logger) error {
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	conf := KServiceConfig()
	conf.Executable = execPath
	//conf.WorkingDirectory = agentDir

	serv, err := kservice.New(nil, conf)
	if err != nil {
		return fmt.Errorf("error creating service, err: %v", err)
	}
	err = serv.Install()
	if err != nil {
		// if we get this error on windows, it's because the user needs Administrator rights to install a Windows service
		if runtime.GOOS == "windows" && strings.Contains(err.Error(), "Access is denied") {
			return errors.New("Access was defined installing the agent as a Windows Service. You will need to re-run this command as Administrator so that the agent can be installed with the right permissions.")
		}
		if strings.Contains(err.Error(), "already exists") {
			logger.Debug("agent already installed, will attempt to start it")
		} else {
			return fmt.Errorf("could not install service, err: %v", err)
		}
	}
	err = serv.Start()
	if err != nil {
		return fmt.Errorf("could not start service, err: %v", err)
	}
	logger.Info("service was started")
	return nil
}

func KServiceConfig() *kservice.Config {
	res := &kservice.Config{
		Name:        "io.pinpt.agent",
		DisplayName: "Pinpoint Agent",
		Description: "The Pinpoint Agent will process data and send to Pinpoint",
		Option: service.KeyValue{
			"RunAtLoad": true,
		},
	}
	if runtime.GOOS != "linux" {
		res.Option["UserService"] = true
	}
	return res
}
