package cmdvalidate

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/pinpt/agent.next/pkg/iloader"
	"github.com/pinpt/agent.next/rpcdef"

	"github.com/pinpt/agent.next/cmd/cmdintegration"
)

type Opts struct {
	cmdintegration.Opts
	Output io.Writer
}

type AgentConfig = cmdintegration.AgentConfig
type Integration = cmdintegration.Integration

func Run(opts Opts) error {
	exp := newValidator(opts)
	defer exp.Destroy()
	return nil
}

type validator struct {
	*cmdintegration.Command

	Opts Opts

	integrationConfig cmdintegration.Integration
	integration       *iloader.Integration
}

func newValidator(opts Opts) *validator {
	s := &validator{}
	if len(opts.Integrations) != 1 {
		panic("pass exactly 1 integration")
	}

	s.Command = cmdintegration.NewCommand(opts.Opts)
	s.Opts = opts

	s.SetupIntegrations(agentDelegate{validator: s})

	s.integrationConfig = opts.Integrations[0]
	s.integration = s.Integrations[s.integrationConfig.Name]

	s.runValidate()
	return s
}

func (s *validator) runValidate() error {
	ctx := context.Background()
	client := s.integration.RPCClient()

	configPinpoint := rpcdef.ExportConfigPinpoint{
		CustomerID: s.Opts.AgentConfig.CustomerID,
	}
	exportConfig := rpcdef.ExportConfig{
		Pinpoint:    configPinpoint,
		Integration: s.integrationConfig.Config,
	}

	res, err := client.ValidateConfig(ctx, exportConfig)
	if err != nil {
		return err
	}

	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = s.Opts.Output.Write(b)
	if err != nil {
		return err
	}

	// BUG: last log message is missing without this
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (s *validator) Destroy() {
}
