package main

import (
	"context"
	"runtime"
	"time"

	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/rpcdef"

	"github.com/hashicorp/go-hclog"
)

type Integration struct {
	logger hclog.Logger
	agent  rpcdef.Agent
}

func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	return nil
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	s.logger.Info("Press CTRL+C to test termination.")
	if runtime.GOOS == "windows" {
		s.logger.Info(`When done run ".\test.exe check" to test if process existed`)
	} else {
		s.logger.Info(`When done run "./dist/local/test check" to test if process existed`)
	}

	time.Sleep(15 * time.Minute)
	return res, nil
}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {
	return res, nil
}

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	res.Error = rpcdef.ErrOnboardExportNotSupported
	return
}

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}
