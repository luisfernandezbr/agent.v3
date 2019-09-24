package cmdservicerun

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/pinpt/agent.next/cmd/cmdupload"

	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/fsconf"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/integration-sdk/agent"
)

type exporterOpts struct {
	Logger       hclog.Logger
	PinpointRoot string
	FSConf       fsconf.Locs
	Conf         agentconf.Config

	PPEncryptionKey string
	AgentConfig     cmdintegration.AgentConfig
}

type exporter struct {
	ExportQueue chan exportRequest

	conf agentconf.Config

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
	s.conf = opts.Conf
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
	s.logger.Info("processing export request", "job_id", data.JobID, "request_date", data.RequestDate.Rfc3339, "reprocess_historical", data.ReprocessHistorical)

	var integrations []cmdexport.Integration

	// add in additional integrations defined in config

	for _, in := range s.conf.ExtraIntegrations {
		integrations = append(integrations, cmdexport.Integration{
			Name:   in.Name,
			Config: in.Config,
		})
	}

	for _, integration := range data.Integrations {

		s.logger.Info("exporting integration", "name", integration.Name, "len(exclusions)", len(integration.Exclusions))

		//s.logger.Debug("integration data", "data", integration.ToMap())

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

	exportLogSender := newExportLogSender(s.logger, s.conf, data.JobID)

	err = s.execExport(ctx, s.opts.AgentConfig, integrations, data.ReprocessHistorical, exportLogSender)
	if err != nil {
		return err
	}

	err = exportLogSender.FlushAndClose()
	if err != nil {
		s.logger.Error("could not send export logs to the server", "err", err)
	}

	s.logger.Info("export finished, running upload")

	err = cmdupload.Run(ctx, s.logger, s.opts.PinpointRoot, *data.UploadURL)
	if err != nil {
		return err
	}
	return nil
}

func (s *exporter) execExport(ctx context.Context, agentConfig cmdexport.AgentConfig, integrations []cmdexport.Integration, reprocessHistorical bool, exportLogWriter io.Writer) error {

	var logWriter io.Writer
	if exportLogWriter == nil {
		logWriter = os.Stdout
	} else {
		logWriter = io.MultiWriter(os.Stdout, exportLogWriter)
	}

	args := []string{"export", "--log-format", "json"}

	if reprocessHistorical {
		args = append(args, "--reprocess-historical=true")
	}

	fs, err := newFsPassedParams(s.opts.FSConf.Temp, []kv{
		{"--agent-config-file", agentConfig},
		{"--integrations-file", integrations},
	})
	if err != nil {
		return err
	}
	args = append(args, fs.Args()...)
	defer fs.Clean()

	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Stdout = logWriter
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type kv struct {
	K string
	V interface{}
}

type fsPassedParams struct {
	args    []kv
	tempDir string
	files   []string
}

func newFsPassedParams(tempDir string, args []kv) (*fsPassedParams, error) {
	s := &fsPassedParams{}
	s.args = args
	s.tempDir = tempDir
	for _, arg := range args {
		loc, err := s.writeFile(arg.V)
		if err != nil {
			return nil, err
		}
		s.files = append(s.files, loc)
	}
	return s, nil
}

func (s *fsPassedParams) writeFile(obj interface{}) (string, error) {
	err := os.MkdirAll(s.tempDir, 0777)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	f, err := ioutil.TempFile(s.tempDir, "")
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.Write(b)
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}

func (s *fsPassedParams) Args() (res []string) {
	for i, kv0 := range s.args {
		k := kv0.K
		v := s.files[i]
		res = append(res, k, v)
	}
	return
}

func (s *fsPassedParams) Clean() error {
	for _, f := range s.files {
		err := os.Remove(f)
		if err != nil {
			return err
		}
	}
	return nil
}
