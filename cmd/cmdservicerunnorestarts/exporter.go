package cmdservicerunnorestarts

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/pinpt/agent.next/cmd/cmdupload"
	"github.com/pinpt/go-common/event"

	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/deviceinfo"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/jsonstore"
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

	IntegrationsDir string
}

type exporter struct {
	ExportQueue chan exportRequest

	conf agentconf.Config

	logger     hclog.Logger
	opts       exporterOpts
	mu         sync.Mutex
	exporting  bool
	deviceInfo deviceinfo.CommonInfo
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
	s.deviceInfo = deviceinfo.CommonInfo{
		CustomerID: s.conf.CustomerID,
		SystemID:   s.conf.SystemID,
		DeviceID:   s.conf.DeviceID,
		Root:       s.opts.PinpointRoot,
	}
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
func (s *exporter) sendExportEvent(ctx context.Context, jobID string, data *agent.ExportResponse, ints []agent.ExportRequestIntegrations, isIncremental []bool) error {
	data.JobID = jobID
	data.RefType = "export"
	data.Type = agent.ExportResponseTypeExport
	for i, in := range ints {
		v := agent.ExportResponseIntegrations{
			IntegrationID: in.ID,
			Name:          in.Name,
			SystemType:    agent.ExportResponseIntegrationsSystemType(in.SystemType),
		}
		if len(isIncremental) != 0 { // only sending this for completed event
			if len(isIncremental) <= i {
				return errors.New("could not check if export was incremental or not, isIncremental array is not of valid length")
			}
			if isIncremental[i] {
				v.ExportType = agent.ExportResponseIntegrationsExportTypeIncremental
			} else {
				v.ExportType = agent.ExportResponseIntegrationsExportTypeHistorical
			}
		}
		data.Integrations = append(data.Integrations, v)
	}
	s.deviceInfo.AppendCommonInfo(data)
	publishEvent := event.PublishEvent{
		Object: data,
		Headers: map[string]string{
			"uuid": s.conf.DeviceID,
		},
	}
	return event.Publish(ctx, publishEvent, s.conf.Channel, s.conf.APIKey)
}

func (s *exporter) sendStartExportEvent(ctx context.Context, jobID string, ints []agent.ExportRequestIntegrations) error {
	data := &agent.ExportResponse{
		State:   agent.ExportResponseStateStarting,
		Success: true,
	}
	return s.sendExportEvent(ctx, jobID, data, ints, nil)
}

func (s *exporter) sendEndExportEvent(ctx context.Context, jobID string, started, ended time.Time, filesize int64, uploadurl *string, ints []agent.ExportRequestIntegrations, isIncremental []bool, err error) error {
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
	return s.sendExportEvent(ctx, jobID, data, ints, isIncremental)
}
func (s *exporter) export(data *agent.ExportRequest) {
	ctx := context.Background()
	started := time.Now()
	if err := s.sendStartExportEvent(ctx, data.JobID, data.Integrations); err != nil {
		s.logger.Error("error sending export response start event", "err", err)
	}
	isIncremental, fileSize, err := s.doExport(ctx, data)
	if err != nil {
		s.logger.Error("export finished with error", "err", err)
	} else {
		s.logger.Info("sent back export result")
	}
	if err := s.sendEndExportEvent(ctx, data.JobID, started, time.Now(), fileSize, data.UploadURL, data.Integrations, isIncremental, err); err != nil {
		s.logger.Error("error sending export response stop event", "err", err)
	}
}
func (s *exporter) doExport(ctx context.Context, data *agent.ExportRequest) (isIncremental []bool, fileSize int64, rerr error) {
	s.logger.Info("processing export request", "job_id", data.JobID, "request_date", data.RequestDate.Rfc3339, "reprocess_historical", data.ReprocessHistorical)

	var integrations []cmdexport.Integration
	// add in additional integrations defined in config
	for _, in := range s.conf.ExtraIntegrations {
		integrations = append(integrations, cmdexport.Integration{
			Name:   in.Name,
			Config: in.Config,
		})
	}

	lastProcessedStore, err := jsonstore.New(s.opts.FSConf.LastProcessedFile)
	if err != nil {
		rerr = err
		return
	}

	for _, integration := range data.Integrations {
		s.logger.Info("exporting integration", "name", integration.Name, "len(exclusions)", len(integration.Exclusions))
		conf, err := configFromEvent(integration.ToMap(), IntegrationType(integration.SystemType), s.opts.PPEncryptionKey)
		if err != nil {
			rerr = err
			return
		}
		integrations = append(integrations, conf)

		if data.ReprocessHistorical {
			isIncremental = append(isIncremental, false)
		} else {
			lastProcessed, err := s.getLastProcessed(lastProcessedStore, conf)
			if err != nil {
				rerr = err
				return
			}
			isIncremental = append(isIncremental, lastProcessed != "")
		}
	}

	fsconf := s.opts.FSConf
	// delete existing uploads
	if err = os.RemoveAll(fsconf.Uploads); err != nil {
		rerr = err
		return
	}

	exportLogSender := newExportLogSender(s.logger, s.conf, data.JobID)
	s.opts.AgentConfig.Backend.ExportJobID = data.JobID
	if err := s.execExport(ctx, integrations, data.ReprocessHistorical, exportLogSender); err != nil {
		rerr = err
		return
	}
	if err := exportLogSender.FlushAndClose(); err != nil {
		s.logger.Error("could not send export logs to the server", "err", err)
		rerr = err
		return
	}

	s.logger.Info("export finished, running upload")
	fileSize, err = cmdupload.Run(ctx, s.logger, s.opts.PinpointRoot, *data.UploadURL)
	if err != nil {
		if err == cmdupload.ErrNoFilesFound {
			s.logger.Info("skipping upload, no files generated")
			// do not return errors when no files to upload, which is ok for inremental
		} else {
			rerr = err
			return
		}
	}
	return
}

func (s *exporter) getLastProcessed(lastProcessed *jsonstore.Store, in cmdexport.Integration) (string, error) {
	id, err := in.ID()
	if err != nil {
		return "", err
	}
	v := lastProcessed.Get(id.String())
	if v == nil {
		return "", nil
	}
	ts, ok := v.(string)
	if !ok {
		return "", errors.New("not a valid value saved in last processed key")
	}
	return ts, nil
}

func (s *exporter) execExport(ctx context.Context, integrations []cmdexport.Integration, reprocessHistorical bool, exportLogWriter io.Writer) error {
	c := &subCommand{
		ctx:          ctx,
		logger:       s.logger,
		tmpdir:       s.opts.FSConf.Temp,
		config:       s.opts.AgentConfig,
		conf:         s.conf,
		integrations: integrations,
		deviceInfo:   s.deviceInfo,
	}
	c.validate()
	if exportLogWriter != nil {
		c.logWriter = &exportLogWriter
	}

	args := []string{
		"--log-level", logutils.LogLevelToString(s.opts.LogLevelSubcommands),
	}
	if reprocessHistorical {
		args = append(args, "--reprocess-historical=true")
	}
	err := c.run("export", nil, args...)
	if err != nil {
		return err
	}
	return err
}
