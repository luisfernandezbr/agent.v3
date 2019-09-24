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

	integration  *iloader.Integration
	exportConfig rpcdef.ExportConfig
}

func newExport(opts Opts) (*export, error) {
	s := &export{}
	if len(opts.Integrations) != 1 {
		panic("pass exactly 1 integration")
	}

	var err error
	s.Command, err = cmdintegration.NewCommand(opts.Opts)
	if err != nil {
		return nil, err
	}
	s.Opts = opts

	s.SetupIntegrations(nil)

	integrationName := opts.Integrations[0].Name
	s.integration = s.Integrations[integrationName]
	s.exportConfig = s.ExportConfigs[integrationName]

	err = s.runExport()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *export) runExport() error {
	ctx := context.Background()
	client := s.integration.RPCClient()

	cmdRes := Result{}

	res, err := client.OnboardExport(ctx, s.Opts.ExportType, s.exportConfig)
	if err != nil {
		cmdRes.Error = err.Error()
	} else {
		if res.Error != nil {
			cmdRes.Error = fmt.Sprintf("could not retrive data for onboard type: %v integration: %v err: %v", s.Opts.ExportType, s.integration.Name(), res.Error.Error())
		}
	}

	if cmdRes.Error == "" {
		cmdRes.Success = true
	}

	cmdRes.Records = res.Records

	b, err := json.Marshal(cmdRes)
	if err != nil {
		return err
	}
	_, err = s.Opts.Output.Write(b)
	if err != nil {
		return err
	}

	s.Logger.Info("export-onboard-data completed", "success", cmdRes.Success, "err", res.Error)

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
