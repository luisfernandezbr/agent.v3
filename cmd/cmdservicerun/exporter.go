package cmdservicerun

import (
	"context"
	"os"
	"os/exec"

	"github.com/pinpt/agent.next/cmd/cmdupload"

	"github.com/pinpt/agent.next/pkg/fsconf"

	pjson "github.com/pinpt/go-common/json"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/integration-sdk/agent"
)

type exporterOpts struct {
	Logger       hclog.Logger
	CustomerID   string
	PinpointRoot string
	FSConf       fsconf.Locs

	PPEncryptionKey string
}

type exporter struct {
	ExportQueue chan exportRequest

	logger hclog.Logger
	opts   exporterOpts
}

type exportRequest struct {
	Done chan error
	Data *agent.ExportRequest
}

func newExporter(opts exporterOpts) *exporter {
	if opts.PPEncryptionKey == "" {
		panic(`opts.PPEncryptionKey == ""`)
	}
	s := &exporter{}
	s.opts = opts
	s.logger = opts.Logger
	s.ExportQueue = make(chan exportRequest)
	return s
}

func (s *exporter) Run() {
	for req := range s.ExportQueue {
		req.Done <- s.export(req.Data)
	}
	return
}

func (s *exporter) export(data *agent.ExportRequest) error {
	s.logger.Info("processing export request", "upload_url", *data.UploadURL)

	agentConfig := cmdexport.AgentConfig{}
	agentConfig.CustomerID = s.opts.CustomerID
	agentConfig.PinpointRoot = s.opts.PinpointRoot

	var integrations []cmdexport.Integration

	/*
		integrations = append(integrations, cmdexport.Integration{
			Name:   "mock",
			Config: map[string]interface{}{"k1": "v1"},
		})
	*/

	for _, integration := range data.Integrations {

		s.logger.Info("exporting integration", "name", integration.Name)

		conf, err := configFromEvent(integration.ToMap(), s.opts.PPEncryptionKey)
		if err != nil {
			return err
		}

		integrations = append(integrations, conf)
	}

	ctx := context.Background()

	fsconf := s.opts.FSConf

	// delete existing uploads
	err := os.RemoveAll(fsconf.Uploads)
	if err != nil {
		return err
	}

	err = s.execExport(ctx, agentConfig, integrations)
	if err != nil {
		return err
	}

	s.logger.Info("export finished, running upload")

	err = cmdupload.Run(ctx, s.logger, s.opts.PinpointRoot, *data.UploadURL)
	if err != nil {
		return err
	}
	return nil
}

func (s *exporter) execExport(ctx context.Context, agentConfig cmdexport.AgentConfig, integrations []cmdexport.Integration) error {
	cmd := exec.CommandContext(ctx, os.Args[0], "export", "--agent-config-json", pjson.Stringify(agentConfig), "--integrations-json", pjson.Stringify(integrations))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
