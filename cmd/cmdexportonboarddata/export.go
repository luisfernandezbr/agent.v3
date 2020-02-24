package cmdexportonboarddata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pinpt/agent/rpcdef"

	"github.com/pinpt/agent/cmd/cmdintegration"
)

type Opts struct {
	cmdintegration.Opts
	Output     io.Writer
	ExportType rpcdef.OnboardExportType
}

func Run(opts Opts) error {
	exp, err := newExport(opts)
	if err != nil {
		return err
	}
	return exp.Destroy()
}

type export struct {
	*cmdintegration.Command

	Opts Opts

	integration cmdintegration.Integration
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

	err = s.SetupIntegrations(nil)
	if err != nil {
		return nil, err
	}

	s.integration = s.OnlyIntegration()

	err = s.runExportAndPrint()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *export) runExportAndPrint() error {
	data, err := s.runExport()
	res := Result{}
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Data = data
		res.Success = true
	}

	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = s.Opts.Output.Write(b)
	if err != nil {
		return err
	}

	s.Logger.Info("export-onboard-data completed", "success", res.Success, "err", res.Error)

	// BUG: last log message is missing without this
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (s *export) runExport() (data interface{}, _ error) {
	ctx := context.Background()
	client := s.integration.ILoader.RPCClient()

	res, err := client.OnboardExport(ctx, s.Opts.ExportType, s.integration.ExportConfig)
	if err != nil {
		_ = s.CloseOnlyIntegrationAndHandlePanic(s.integration.ILoader)
		return nil, err
	}
	if res.Error != nil {
		return nil, fmt.Errorf("could not retrive data for onboard type: %v integration: %v err: %v", s.Opts.ExportType, s.integration.Export.String(), res.Error.Error())
	}

	err = s.CloseOnlyIntegrationAndHandlePanic(s.integration.ILoader)
	if err != nil {
		return nil, fmt.Errorf("error closing integration, err: %v", err)
	}

	return res.Data, nil
}

type Result struct {
	Success bool        `json:"success"`
	Error   string      `json:"error"`
	Data    interface{} `json:"data"`
}

type DataRepos []map[string]interface{}
type DataProjects []map[string]interface{}
type DataWorkConfig map[string]interface{}

type DataUsers struct {
	Users []map[string]interface{} `json:"users"`
	Teams []map[string]interface{} `json:"teams"`
}

func (s *export) Destroy() error {
	return nil
}
