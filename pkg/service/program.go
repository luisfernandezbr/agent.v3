package service

import (
	"fmt"
	"os"
	"runtime"

	"github.com/kardianos/service"
)

// ErrUninstallExit uninstall error
var ErrUninstallExit = fmt.Errorf("uninstall exit")

type Opts struct {
	PinpointRoot string
}

type program struct {
	logger service.Logger

	terminate  chan bool
	terminated chan bool
	endRunFunc chan bool

	serviceFunc func(cancel chan bool) error

	name string

	names Names

	opts Opts

	uninstallTriggered bool
}

func newProgram(name string, opts Opts, serviceFunc func(cancel chan bool) error) *program {
	p := &program{}
	p.serviceFunc = serviceFunc
	p.name = name
	p.opts = opts
	return p
}

// SetLogger sets the logger that is initialized from the service.
// Need to set it later to avoid circular dependency.
func (p *program) SetLogger(l service.Logger) {
	p.logger = l
}

func (p *program) Start(s service.Service) error {
	p.logger.Info("starting service")
	go func() {
		err := p.run()
		if err != nil {
			p.logger.Error("error in run", "err", err)
		}
	}()
	return nil
}

func (p *program) Stop(s service.Service) (err error) {

	p.logger.Info("stopping service")

	if err = p.deletePIDFileIfLinux(); err != nil {
		return err
	}

	if p.uninstallTriggered {
		opts := UninstallOpts{}
		opts.PrintLog = func(msg string, args ...interface{}) {
			p.logger.Info(msg, args)
		}
		if err := UninstallAndDelete(opts, p.opts.PinpointRoot); err != nil {
			return fmt.Errorf("error on uninstall, err: %v", err)
		}
	} else {
		p.terminate <- true
		<-p.terminated
	}

	p.logger.Info("service stopped")

	return
}

func (p *program) deletePIDFileIfLinux() (err error) {

	if runtime.GOOS == "linux" {
		pid := fmt.Sprintf("/var/run/%s.pid", p.name)
		p.logger.Info("deleting file", pid)
		// This is needed for alpine only
		os.Remove(pid)
	}

	return
}

func (p *program) run() (_ error) {
	p.logger.Info("running service")

	p.endRunFunc = make(chan bool)

	p.terminate = make(chan bool)
	// if terminated via kill, Stop function will not be called
	// for this reason need to use chan len 1
	p.terminated = make(chan bool, 1)

	cleanup := func() {
		if !p.uninstallTriggered {
			p.terminated <- true
		}
	}

	defer cleanup()

	rerr := func(err error) {
		p.logger.Error(err)
	}

	err := p.serviceFunc(p.terminate)
	if err != nil && err != ErrUninstallExit {
		rerr(fmt.Errorf("error in run: %v", err))
		return
	}

	if err == ErrUninstallExit {

		p.uninstallTriggered = true

		go func() {
			config := p.names.serviceConfig()
			svc, err := service.New(nil, config)
			if err != nil {
				p.logger.Error("error getting service", err)
				return
			}
			if err = svc.Stop(); err != nil && err.Error() != "exit status 1" {
				p.logger.Error("error on stop", "err", err)
			}
			p.endRunFunc <- true
		}()

		<-p.endRunFunc

		// NOT POSSIBLE TO SEND UNINSTALL RESPONSE HERE (The apikey was eliminated)
		// Maybe make the uninstall response public
		// as the EnrollRequest
	}

	p.logger.Info("os service run func done")

	return nil
}
