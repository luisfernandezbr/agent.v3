package cmdexportonboarddata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pinpt/agent.next/pkg/iloader"
	"github.com/pinpt/agent.next/rpcdef"

	"github.com/pinpt/agent.next/cmd/cmdintegration"
)

type Opts struct {
	cmdintegration.Opts
	Output     io.Writer
	ExportType rpcdef.OnboardExportType
}

type AgentConfig = cmdintegration.AgentConfig
type Integration = cmdintegration.Integration

func Run(opts Opts) error {
	exp, err := newExport(opts)
	if err != nil {
		return err
	}
	defer exp.Destroy()
	return nil
}

type export struct {
	*cmdintegration.Command

	Opts Opts

	integrationConfig cmdintegration.Integration
	integration       *iloader.Integration
}

func newExport(opts Opts) (*export, error) {
	s := &export{}
	if len(opts.Integrations) != 1 {
		panic("pass exactly 1 integration")
	}

	s.Command = cmdintegration.NewCommand(opts.Opts)
	s.Opts = opts

	s.SetupIntegrations(agentDelegate{export: s})

	s.integrationConfig = opts.Integrations[0]
	s.integration = s.Integrations[s.integrationConfig.Name]

	err := s.runExport()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *export) runExport() error {
	ctx := context.Background()
	client := s.integration.RPCClient()

	configPinpoint := rpcdef.ExportConfigPinpoint{
		CustomerID: s.Opts.AgentConfig.CustomerID,
	}
	exportConfig := rpcdef.ExportConfig{
		Pinpoint:    configPinpoint,
		Integration: s.integrationConfig.Config,
	}

	res, err := client.OnboardExport(ctx, s.Opts.ExportType, exportConfig)
	if err != nil {
		return err
	}

	res2 := Result{}
	if res.Error == nil {
		res2.Success = true
	} else {
		res2.Error = fmt.Sprintf("could not retrive data for onboard type: %v integration: %v err: %v", s.Opts.ExportType, s.integration.Name(), res.Error.Error())
	}
	res2.Records = res.Records

	b, err := json.Marshal(res2)
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

type Result struct {
	Success bool                     `json:"success"`
	Error   string                   `json:"error"`
	Records []map[string]interface{} `json:"records"`
}

func (s *export) Destroy() {
}
