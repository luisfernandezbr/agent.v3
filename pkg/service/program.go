package service

import (
	"bufio"
	"fmt"
	"os"
	"runtime"

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

	var doneStdoutProxy chan bool

	cleanup := func() {
		if doneStdoutProxy != nil {
			if err := os.Stdout.Sync(); err != nil {
				p.logger.Info("error closing writeFile", "err", err)
			}
			<-doneStdoutProxy
		}
		p.terminated <- true
	}

	defer cleanup()

	rerr := func(err error) {
		p.logger.Error(err)
	}

	if runtime.GOOS == "windows" {
		var err error
		doneStdoutProxy, err = p.proxyStdoutIntoLogger()
		if err != nil {
			rerr(fmt.Errorf("could not create output to log proxy: %v", err))
			return
		}
	}

	err := p.serviceFunc(p.terminate)
	if err != nil {
		rerr(fmt.Errorf("error in run: %v", err))
		return
	}

	return nil
}

func (p *program) proxyStdoutIntoLogger() (done chan bool, rerr error) {
	done = make(chan bool)

	var readFile *os.File
	var writeFile *os.File

	var err error
	readFile, writeFile, err = os.Pipe()
	if err != nil {
		rerr = err
		return
	}

	os.Stdout = writeFile

	go func() {
		scanner := bufio.NewScanner(readFile)
		for scanner.Scan() {
			line := scanner.Text()
			// BUG: this logs all logs as info, even though we could have different underlying error levels
			p.logger.Info(line)
		}
		err := scanner.Err()
		if err != nil {
			p.logger.Error("error on scanner", "err", err)
		}
		done <- true
	}()

	return
}
