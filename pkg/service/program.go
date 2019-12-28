package service

import (
	"bufio"
	"context"
	"os"
	"runtime"

	"github.com/kardianos/service"
	"github.com/pinpt/agent/cmd/cmdrun"
)

type program struct {
	ctx             context.Context
	opts            cmdrun.Opts
	serviceLogger   service.Logger
	terminateCmd    chan bool
	serviceFinished chan bool
}

func (p *program) Start(s service.Service) error {
	p.serviceLogger.Info("service start")
	go func() {
		err := p.run()
		if err != nil {
			p.serviceLogger.Error("error on run", "err", err)
		}
	}()
	return nil
}

func (p *program) run() error {
	p.serviceLogger.Info("service running")

	p.terminateCmd = make(chan bool, 1)
	p.serviceFinished = make(chan bool, 1)

	doneScanner := make(chan bool, 1)
	var readFile *os.File
	var writeFile *os.File

	if runtime.GOOS == "windows" {
		var err error
		readFile, writeFile, err = os.Pipe()
		if err != nil {
			return err
		}

		os.Stdout = writeFile

		go func() {
			scanner := bufio.NewScanner(readFile)
			for scanner.Scan() {
				line := scanner.Text()
				p.serviceLogger.Info(line)
			}

			err := scanner.Err()
			if err != nil {
				p.serviceLogger.Error("error on scanner", "err", err)
			}
			doneScanner <- true
		}()
	}

	err := cmdrun.Run(p.ctx, p.opts, p.terminateCmd)
	if err != nil {
		p.serviceLogger.Error("error on run command", "err", err)
	}

	if runtime.GOOS == "windows" {
		if err := writeFile.Close(); err != nil {
			p.serviceLogger.Info("error closing writeFile", "err", err)
		}

		<-doneScanner
	}

	p.serviceFinished <- true

	return nil
}
func (p *program) Stop(s service.Service) error {
	p.serviceLogger.Info("service stopped")
	p.terminateCmd <- true
	<-p.serviceFinished
	p.serviceLogger.Info("service finished")
	return nil
}
