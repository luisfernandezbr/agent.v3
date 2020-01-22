package service

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/kardianos/service"
)

type ControlAction string

const (
	Uninstall ControlAction = "uninstall"
	Start                   = "start"
	Stop                    = "stop"
	Restart                 = "restart"
	Status                  = "status"
)

func Install(logger hclog.Logger, names Names, runArgs []string) error {
	logger.Info("installing service")
	config := names.serviceConfig()
	config.Arguments = runArgs
	svc, err := service.New(nil, config)
	if err != nil {
		return err
	}
	err = service.Control(svc, "install")
	// TODO: check if we can ignore status 1
	if err != nil && !strings.Contains(err.Error(), "exit status 1") {
		return err
	}
	return nil
}

func Control(logger hclog.Logger, names Names, action ControlAction) error {
	logger.Info("service-control", "action", string(action))

	config := names.serviceConfig()
	svc, err := service.New(nil, config)
	if err != nil {
		return err
	}
	if action == Status {
		return status(logger, svc)
	}

	err = service.Control(svc, string(action))
	// TODO: check if we can ignore status 1
	if err != nil && !strings.Contains(err.Error(), "exit status 1") {
		return err
	}
	return nil
}

func status(logger hclog.Logger, svc service.Service) error {
	status, err := svc.Status()
	if err != nil {
		return fmt.Errorf("could not determine service status, err: %v", err)
	}
	switch status {
	case service.StatusUnknown:
		logger.Warn("Can't determine status. Agent is not installed or some other error encountered.")
	case service.StatusRunning:
		logger.Info("Agent running.")
	case service.StatusStopped:
		logger.Info("Agent stopped.")
	}
	return nil
}

func Run(names Names, serviceFunc func(cancel chan bool) error) {

	prg := newProgram(serviceFunc)
	config := names.serviceConfig()

	svc, err := service.New(prg, config)
	if err != nil {
		panic(err)
	}
	logger, err := createLoggerFromService(svc)
	if err != nil {
		panic(err)
	}
	prg.SetLogger(logger)
	err = svc.Run()
	if err != nil {
		logger.Error(err)
		return
	}
	return
}

func createLoggerFromService(svc service.Service) (_ service.Logger, rerr error) {
	errs := make(chan error)
	logger, err := svc.Logger(errs)
	if err != nil {
		rerr = err
		return
	}
	go func() {
		for {
			err := <-errs
			if err != nil {
				logger.Error("logger error", "err", err)
			}
		}
	}()
	return logger, nil
}

type Names struct {
	Name        string
	DisplayName string
	Description string
}

func (s Names) serviceConfig() *service.Config {
	res := &service.Config{
		Option: service.KeyValue{
			"RunAtLoad": true,
		},
	}
	res.Name = s.Name
	res.DisplayName = s.DisplayName
	res.Description = s.Description
	if runtime.GOOS != "linux" {
		res.Option["UserService"] = true
	}
	if runtime.GOOS == "windows" {
		res.Dependencies = []string{"RpcSs"}
	}
	return res
}
