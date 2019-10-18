package cmdservicerun

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
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

	logger    hclog.Logger
	opts      exporterOpts
	mu        sync.Mutex
	exporting bool
}

type exportRequest struct {
	Done chan bool
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
		s.SetRunning(true)
		s.export(req.Data)
		s.SetRunning(false)
		req.Done <- true
	}
	return
}

func (s *exporter) SetRunning(ex bool) {
	s.mu.Lock()
	s.exporting = ex
	s.mu.Unlock()

}
func (s *exporter) IsRunning() bool {
	s.mu.Lock()
	ex := s.exporting
	s.mu.Unlock()
	return ex
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

	deviceinfo.AppendCommonInfoFromConfig(data, s.conf)
	publishEvent := event.PublishEvent{
		Object: data,
		Headers: map[string]string{
			"uuid": s.conf.DeviceID,
		},
	}
	return event.Publish(ctx, publishEvent, s.conf.Channel, s.conf.APIKey)
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

func (s *exporter) sendEndExportEvent(ctx context.Context, jobID string, started, ended time.Time, filesize int64, uploadurl *string, ints []agent.ExportRequestIntegrations, err error) error {
	if !s.opts.AgentConfig.Backend.Enable {
		return nil
	}
	data := &agent.ExportResponse{
		State:     agent.ExportResponseStateCompleted,
		Size:      filesize,
		UploadURL: uploadurl,
	}
	date.ConvertToModel(started, &data.StartDate)
	date.ConvertToModel(ended, &data.EndDate)
	if err != nil {
		errstr := err.Error()
		data.Error = &errstr
		data.Success = false
	} else {
		data.Success = true
	}
	return s.sendExportEvent(ctx, jobID, data, ints)
}
func (s *exporter) export(data *agent.ExportRequest) {
	ctx := context.Background()
	started := time.Now()
	if err := s.sendStartExportEvent(ctx, data.JobID, data.Integrations); err != nil {
		s.logger.Error("error sending export response start event", "err", err)
	}
	fileSize, err := s.doExport(ctx, data)
	if err != nil {
		s.logger.Error("export finished with error", "err", err)
	} else {
		s.logger.Info("sent back export result")
	}
	if err := s.sendEndExportEvent(ctx, data.JobID, started, time.Now(), fileSize, data.UploadURL, data.Integrations, err); err != nil {
		s.logger.Error("error sending export response stop event", "err", err)
	}
}
func (s *exporter) doExport(ctx context.Context, data *agent.ExportRequest) (fileSize int64, err error) {
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
		conf, err := configFromEvent(integration.ToMap(), IntegrationType(integration.SystemType), s.opts.PPEncryptionKey)
		if err != nil {
			return 0, err
		}
		integrations = append(integrations, conf)
	}
	fsconf := s.opts.FSConf
	// delete existing uploads
	if err = os.RemoveAll(fsconf.Uploads); err != nil {
		return 0, err
	}
	exportLogSender := newExportLogSender(s.logger, s.conf, data.JobID)
	agentConfig := s.opts.AgentConfig
	agentConfig.Backend.ExportJobID = data.JobID
	if err := s.execExport(ctx, agentConfig, integrations, data.ReprocessHistorical, exportLogSender); err != nil {
		return 0, err
	}
	if err := exportLogSender.FlushAndClose(); err != nil {
		s.logger.Error("could not send export logs to the server", "err", err)
		return 0, err
	}
	s.logger.Info("export finished, running upload")
	url := "https://pinpt-134727976553-batch-bucket.s3.amazonaws.com/pinpt/a43b13675cceae50/1571415630023-4e9eb78e8336bad9ef0d120a/pinpt-upload-1571415630023-4e9eb78e8336bad9ef0d120a.zip?Content-Type=application%2Fzip&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAR6XTJYZU4S3SKPWT%2F20191018%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20191018T162030Z&X-Amz-Expires=86400&X-Amz-Security-Token=AgoJb3JpZ2luX2VjEJn%2F%2F%2F%2F%2F%2F%2F%2F%2F%2FwEaCXVzLWVhc3QtMSJGMEQCIBE4a%2BenlQQOJR75xr7yK4SAYIyZ3W9IkkrevZvXJKLkAiAMZHOBAexOPR0NhBzaNfXwdU4rSwPFgKnA9K2Baq8jZCrjAwiR%2F%2F%2F%2F%2F%2F%2F%2F%2F%2F8BEAEaDDEzNDcyNzk3NjU1MyIMiSRT9dmqxly0GhqyKrcDoBpS6secHLHdQaWT9N3pTROp7lJ%2FiTMxmGcQMeon8uMe2Htp9wqTuE5WJ8iPmwVMxyifXUbdPwpDWvHMUTKlIDmId47kcgVRglByOhXRpr5NTCJsAZ3O%2B3HSin7usI5U2USXBszDKq0GbWUS2zdA0pAvIiNz8p2oRxg%2BPT4rviYR%2FRJRDaiXNqaxSGC%2BAwJR1F9eLZrOVFC%2BEyi8uQIDwKhWFmgIE4BqVDetPwZP1SN7kin5%2BaYyx9YM7mf5KSPVD09%2B%2FSAz3Vp7kGbPc%2BZmENHaqqmVjM6TIc%2BAVrbrpT3ukRcHoSV6lUpwkArUlCAmxGQFgz9WveRMejiV0vja8f9cEe4HfmFu5x84CioJBrX8kE03lzT2LHvAmIT9J4s3MOcLaeYungdSymIJhKpge2XuNYD7oTJaBp9tMwHwFn0pqnm5IMydZMiEmGjBL%2BDjy8EvVeydInfKWCkTahTThmJcKsMtreqveySRDVu5oelsnquXneF6ZTVJ5NgBtCPS6a7MKVTRReOv8RssaX0oMwm7rvi2aSOf1nUCknSP7NRaRaf3O1AaZcoupPwdcY4PxZAKJJVvqTDCyKftBTq1AUV2aysrHhITiDm6fikCkSKruuybkX3CMhNQYQy8%2Fe5TYIk5EkaUNAtaDZnFWfYOk5st4BvaYxZ0BQGMs5e58EYmIY431lLcg0dyzkAswthC1FCL6vJA24vTHBzd4KGcvbVBESyuvMY79xFFpSdyawWXYH8tscgDOikBwkqArzgvNZVYzsQf7CmFx0uQaQVRX1FIUVKP9tD333tDrCwqkAGBB76oTvKEMR91nm9MpouAdtNwxG0%3D&X-Amz-Signature=880654692a4b17d080091b069e304522b54427acdab9df702412e1cadd9fdaa5&X-Amz-SignedHeaders=host"
	if fileSize, err = cmdupload.Run(ctx, s.logger, s.opts.PinpointRoot, url); err != nil {
		return 0, err
	}
	return fileSize, nil
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
