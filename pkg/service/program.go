package service

import (
	"fmt"

	"github.com/kardianos/service"
)

type program struct {
	logger     service.Logger
	terminate  chan bool
	terminated chan bool

	serviceFunc func(cancel chan bool) error
}

func newProgram(serviceFunc func(cancel chan bool) error) *program {
	p := &program{}
	p.serviceFunc = serviceFunc
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

func (p *program) Stop(s service.Service) error {
	p.logger.Info("stopping service")
	p.terminate <- true
	<-p.terminated
	p.logger.Info("service stopped")
	return nil
}

func (p *program) run() (_ error) {
	p.logger.Info("running service")

	p.terminate = make(chan bool)
	p.terminated = make(chan bool)

	cleanup := func() {
		p.terminated <- true
	}

	defer cleanup()

	rerr := func(err error) {
		p.logger.Error(err)
	}

	err := p.serviceFunc(p.terminate)
	if err != nil {
		rerr(fmt.Errorf("error in run: %v", err))
		return
	}

	return nil
}
