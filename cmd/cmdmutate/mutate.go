package cmdmutate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pinpt/agent/pkg/iloader"
	"github.com/pinpt/agent/rpcdef"

	"github.com/pinpt/agent/cmd/cmdintegration"
)

type Mutation struct {
	// Fn is the name of the mutation function
	Fn string `json:"mutation"`
	// Data contains mutation parameters as json
	Data interface{} `json:"data"`
}

type Result struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type Opts struct {
	cmdintegration.Opts
	Output   io.Writer
	Mutation Mutation
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

	err = s.SetupIntegrations(nil)
	if err != nil {
		return nil, err
	}

	s.integration = s.Integrations[0]
	s.exportConfig = s.ExportConfigs[0]

	err = s.runAndPrint()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *export) runAndPrint() error {
	err := s.run()

	res := Result{}
	if err != nil {
		res.Error = err.Error()
	} else {
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

	s.Logger.Info("mutate completed", "success", res.Success, "err", res.Error)

	// BUG: last log message is missing without this
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *export) run() error {
	ctx := context.Background()
	client := s.integration.RPCClient()

	data, err := json.Marshal(s.Opts.Mutation.Data)
	if err != nil {
		return err
	}
	err = client.Mutate(ctx, s.Opts.Mutation.Fn, string(data), s.exportConfig)
	if err != nil {
		_ = s.CloseOnlyIntegrationAndHandlePanic(s.integration)
		return fmt.Errorf("could not execute mutation: %v %v err: %v", s.IntegrationIDs[0], s.Opts.Mutation.Fn, err)
	}
	err = s.CloseOnlyIntegrationAndHandlePanic(s.integration)
	if err != nil {
		return fmt.Errorf("error closing integration, err: %v", err)
	}
	return nil
}

func (s *export) Destroy() error {
	return nil
}
