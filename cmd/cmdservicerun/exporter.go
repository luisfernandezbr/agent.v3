package cmdservicerun

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/pinpt/agent.next/cmd/cmdupload"
	"github.com/pinpt/go-common/event"

	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/deviceinfo"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/logutils"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/integration-sdk/agent"
)

// IntegrationType is the enumeration type for system_type
type IntegrationType int32

const (
	// IntegrationTypeWork is the enumeration value for work
	IntegrationTypeWork IntegrationType = 0
	// IntegrationTypeSourcecode is the enumeration value for sourcecode
	IntegrationTypeSourcecode IntegrationType = 1
	// IntegrationTypeCodequality is the enumeration value for codequality
	IntegrationTypeCodequality IntegrationType = 2
)

// String returns the string value for IntegrationSystemType
func (v IntegrationType) String() string {
	switch int32(v) {
	case 0:
		return "WORK"
	case 1:
		return "SOURCECODE"
	case 2:
		return "CODEQUALITY"
	}
	return "unset"
}

type exporterOpts struct {
	Logger hclog.Logger
	// LogLevelSubcommands specifies the log level to pass to sub commands.
	// Pass the same as used for logger.
	// We need it here, because there is no way to get it from logger.
	LogLevelSubcommands hclog.Level

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

func (s *exporter) sendExportEvent(ctx context.Context, jobID string, data *agent.ExportResponse, ints []agent.ExportRequestIntegrations) error {
	data.JobID = jobID
	data.RefType = "export"
	data.Type = agent.ExportResponseTypeExport
	for _, i := range ints {
		data.Integrations = append(data.Integrations, agent.ExportResponseIntegrations{
			IntegrationID: i.ID, // i.RefID ?
			Name:          i.Name,
			SystemType:    agent.ExportResponseIntegrationsSystemType(i.SystemType),
			// ExportType:    agent.ExportResponseIntegrationsExportTypeHistorical or TypeIncremental,
		})
	}
	enrollConf, err := agentconf.Load(s.opts.FSConf.Config2)
	if err != nil {
		return err
	}

	deviceinfo.AppendCommonInfoFromConfig(data, enrollConf)
	publishEvent := event.PublishEvent{
		Object: data,
		Headers: map[string]string{
			"uuid": enrollConf.DeviceID,
		},
	}
	return event.Publish(ctx, publishEvent, enrollConf.Channel, enrollConf.APIKey)
}

func (s *exporter) sendStartExportEvent(ctx context.Context, jobID string, ints []agent.ExportRequestIntegrations) error {
	if !s.opts.AgentConfig.Backend.Enable {
		return nil
	}
	data := &agent.ExportResponse{
		State:   agent.ExportResponseStateStarting,
		Success: true,
	}
	return s.sendExportEvent(ctx, jobID, data, ints)
}

func (s *exporter) sendEndExportEvent(ctx context.Context, jobID string, ints []agent.ExportRequestIntegrations, success bool, started, ended time.Time) error {
	if !s.opts.AgentConfig.Backend.Enable {
		return nil
	}
	data := &agent.ExportResponse{
		JobID:   jobID,
		RefType: "export",
		State:   agent.ExportResponseStateCompleted,
		Success: true,
	}
	date.ConvertToModel(started, &data.StartDate)
	date.ConvertToModel(ended, &data.EndDate)

	return s.sendExportEvent(ctx, jobID, data, ints)
}

func (s *exporter) export(data *agent.ExportRequest) error {
	ctx := context.Background()
	started := time.Now()
	s.logger.Info("processing export request", "job_id", data.JobID, "request_date", data.RequestDate.Rfc3339, "reprocess_historical", data.ReprocessHistorical)
	err := s.sendStartExportEvent(ctx, data.JobID, data.Integrations)
	if err != nil {
		s.logger.Error("error sending export response start event", "err", err)
	}
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

		conf, err := configFromEvent(integration.ToMap(), IntegrationType(integration.SystemType), s.opts.PPEncryptionKey)
		if err != nil {
			return err
		}

		integrations = append(integrations, conf)
	}

	fsconf := s.opts.FSConf

	// delete existing uploads
	if err = os.RemoveAll(fsconf.Uploads); err != nil {
		return err
	}

	exportLogSender := newExportLogSender(s.logger, s.conf, data.JobID)

	agentConfig := s.opts.AgentConfig
	agentConfig.Backend.ExportJobID = data.JobID

	err = s.execExport(ctx, agentConfig, integrations, data.ReprocessHistorical, exportLogSender)
	if err != nil {
		e := s.sendEndExportEvent(ctx, data.JobID, data.Integrations, false, started, time.Now())
		if e != nil {
			s.logger.Error("error sending export response stop event (1)", "err", e)
		}
		return err
	}

	err = exportLogSender.FlushAndClose()
	if err != nil {
		e := s.sendEndExportEvent(ctx, data.JobID, data.Integrations, false, started, time.Now())
		if e != nil {
			s.logger.Error("error sending export response stop event (2)", "err", e)
		}
		s.logger.Error("could not send export logs to the server", "err", err)
		return err
	}

	s.logger.Info("export finished, running upload")

	err = cmdupload.Run(ctx, s.logger, s.opts.PinpointRoot, *data.UploadURL)
	if err != nil {
		e := s.sendEndExportEvent(ctx, data.JobID, data.Integrations, false, started, time.Now())
		if e != nil {
			s.logger.Error("error sending export response stop event (3)", "err", e)
		}
		return err
	}
	e := s.sendEndExportEvent(ctx, data.JobID, data.Integrations, true, started, time.Now())
	if e != nil {
		s.logger.Error("error sending export response stop event (4)", "err", e)
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

	args := []string{
		"export",
		"--log-format", "json",
		"--log-level", logutils.LogLevelToString(s.opts.LogLevelSubcommands),
	}

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
