package service

import (
	"context"
	"runtime"
	"strings"

	"github.com/kardianos/service"
	"github.com/pinpt/agent/cmd/cmdrun"
)

// Action type
type Action int

const (
	// Install action
	Install Action = iota
	// Uninstall action
	Uninstall
	// Start action
	Start
	// Stop action
	Stop
	// Restart action
	Restart
	// Status action
	Status
	// RunS action
	RunS
)

func (a Action) String() string {
	return [...]string{"install", "uninstall", "start", "stop", "restart", "status", "run"}[a]
}

func Run(action Action, ctx context.Context, opts cmdrun.Opts) error {

	logger := opts.Logger

	logger.Info("service-control", "action", action.String())

	svcConfig := ServiceConfig(opts.PinpointRoot, opts.IntegrationsDir)

	prg := &program{ctx: ctx, opts: opts}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}
	errs := make(chan error, 5)
	l, err := s.Logger(errs)
	if err != nil {
		return err
	}

	prg.serviceLogger = l

	go func() {
		for {
			err := <-errs
			if err != nil {
				logger.Error("error on run command", "err", err)
			}
		}
	}()

	switch action {
	case Install, Uninstall, Start, Stop, Restart:
		err := service.Control(s, action.String())
		if err != nil && !strings.Contains(err.Error(), "exit status 1") {
			l.Error(err)
			return err
		}
	case Status:
		status, err := s.Status()
		if err != nil {
			l.Error(err)
			return err
		}
		switch status {
		case service.StatusUnknown:
			logger.Info("status is unable to be determined due to an error or it was not installed")
		case service.StatusRunning:
			logger.Info("agent running")
		case service.StatusStopped:
			logger.Info("agent stopped")
		}
	case RunS:
		err = s.Run()
		if err != nil {
			l.Error(err)
			return err
		}
	}

	return nil
}

func ServiceConfig(root, integrationsDir string) *service.Config {
	res := &service.Config{
		Name:        "io.pinpt.agent",
		DisplayName: "Pinpoint Agent",
		Description: "The Pinpoint Agent will process data and send to Pinpoint",
		Arguments:   []string{"service-run", "--pinpoint-root", root, "--integrations-dir", integrationsDir},
		Option: service.KeyValue{
			"RunAtLoad": true,
		},
	}
	if runtime.GOOS != "linux" {
		res.Option["UserService"] = true
	}
	if runtime.GOOS == "windows" {
		res.Dependencies = []string{"RpcSs"}
	}

	return res
}
