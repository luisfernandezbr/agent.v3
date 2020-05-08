package cmdwebhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pinpt/agent/pkg/jsonstore"
	"github.com/pinpt/agent/rpcdef"

	"github.com/pinpt/agent/cmd/cmdintegration"
	"github.com/pinpt/agent/cmd/pkg/directexport"
)

type Data struct {
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

type Result struct {
	MutatedObjects rpcdef.MutatedObjects `json:"mutated_objects"`
	Success        bool                  `json:"success"`
	Error          string                `json:"error"`
}

type Opts struct {
	cmdintegration.Opts
	Output io.Writer
	Data   Data
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

	exporter *directexport.RepoExporter

	lastProcessed *jsonstore.Store
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

	s.lastProcessed, err = jsonstore.New(s.Locs.LastProcessedFile)
	if err != nil {
		return nil, err
	}

	s.exporter = directexport.NewRepoExporter(directexport.RepoExporterOpts{
		Logger:        s.Logger,
		AgentConfig:   opts.AgentConfig,
		LastProcessed: s.lastProcessed,
		Locs:          s.Locs,
	})

	err = s.SetupIntegrations(directexport.AgentDelegateFactory(s.Logger, s))
	if err != nil {
		return nil, err
	}

	s.integration = s.OnlyIntegration()

	err = s.runAndPrint()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *export) ExportGitRepo(fetch rpcdef.GitRepoFetch) error {
	return s.exporter.ExportGitRepo(fetch)
}
func (s *export) runAndPrint() error {
	res0, err := s.run()

	res := Result{}
	if err != nil {
		res.Error = err.Error()
	} else if res0.Error != "" {
		res.Error = res0.Error
	} else {
		res.Success = true
		res.MutatedObjects = res0.MutatedObjects
	}
	// add more context
	if res.Error != "" {
		res.Error = fmt.Sprintf("%v (%v)", res.Error, s.integration.Export.IntegrationDef.Name)
	}

	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = s.Opts.Output.Write(b)
	if err != nil {
		return err
	}

	s.Logger.Info("webhook completed", "success", res.Success, "err", res.Error)

	// BUG: last log message is missing without this
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *export) run() (_ rpcdef.WebhookResult, rerr error) {
	ctx := context.Background()
	client := s.integration.ILoader.RPCClient()

	body, err := json.Marshal(s.Opts.Data.Body)
	if err != nil {
		rerr = err
		return
	}

	gitExportRes := make(chan directexport.RepoExporterRes)
	go func() {
		gitExportRes <- s.exporter.Run()
	}()

	res, err := client.Webhook(ctx, s.Opts.Data.Headers, string(body), s.integration.ExportConfig)
	if err != nil {
		_ = s.CloseOnlyIntegrationAndHandlePanic(s.integration.ILoader)
		rerr = err
		return
	}
	err = s.CloseOnlyIntegrationAndHandlePanic(s.integration.ILoader)
	if err != nil {
		rerr = fmt.Errorf("error closing integration, err: %v", err)
		return
	}

	s.exporter.Done()

	s.Logger.Info("waiting for git processing to finish")
	gitRes := <-gitExportRes

	err = gitRes.Err
	if err != nil {
		rerr = fmt.Errorf("git processing failed, err: %v", err)
		return
	} else {
		s.Logger.Debug("git processing finished without errors")
	}
	if res.MutatedObjects == nil {
		res.MutatedObjects = rpcdef.MutatedObjects{}
	}
	for k, v := range gitRes.Data {
		res.MutatedObjects[k] = append(res.MutatedObjects[k], v...)
	}

	err = s.lastProcessed.Save()
	if err != nil {
		s.Logger.Error("could not save updated last_processed file", "err", err)
		rerr = err
		return
	}

	return res, nil
}

func (s *export) Destroy() error {
	return nil
}
