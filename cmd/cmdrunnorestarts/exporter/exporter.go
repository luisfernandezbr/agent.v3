package exporter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pinpt/agent/cmd/cmdintegration"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/exporter/fsqueue"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/subcommand"
	"github.com/pinpt/agent/cmd/cmdupload"

	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/deviceinfo"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/jsonstore"
	"github.com/pinpt/agent/pkg/logutils"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdexport"
	"github.com/pinpt/integration-sdk/agent"
)

// Opts are the options for Exporter
type Opts struct {
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

// Exporter schedules and executes exports
type Exporter struct {
	// ExportQueue for queuing the exports
	// Exports happen serially, with only one happening at once
	ExportQueue chan Request

	conf agentconf.Config

	logger     hclog.Logger
	opts       Opts
	mu         sync.Mutex
	exporting  bool
	deviceInfo deviceinfo.CommonInfo

	queue                 *fsqueue.Queue
	queueRequestForwarder chan fsqueue.Request
}

// Request is the export request to put into the ExportQueue
type Request struct {
	// Data is the ExportRequest data received from the server
	Data *agent.ExportRequest
	// MessageID is the message id received from the server in headers
	MessageID string
}

// New creates exporter
func New(opts Opts) (*Exporter, error) {
	if opts.PPEncryptionKey == "" {
		return nil, errors.New(`opts.PPEncryptionKey == ""`)
	}
	s := &Exporter{}
	s.opts = opts
	s.conf = opts.Conf
	s.deviceInfo = deviceinfo.CommonInfo{
		CustomerID: s.conf.CustomerID,
		SystemID:   s.conf.SystemID,
		DeviceID:   s.conf.DeviceID,
		Root:       s.opts.PinpointRoot,
	}
	s.logger = opts.Logger
	s.ExportQueue = make(chan Request)
	var err error
	s.queue, s.queueRequestForwarder, err = fsqueue.New(opts.Logger, s.opts.FSConf.ExportQueueFile)
	if err != nil {
		return nil, fmt.Errorf("could not create fsqueue: %v", err)
	}
	return s, nil
}

func (s *Exporter) setRunning(ex bool) {
	s.mu.Lock()
	s.exporting = ex
	s.mu.Unlock()
}

// IsRunning returns true if there is an export in progress
func (s *Exporter) IsRunning() bool {
	s.mu.Lock()
	ex := s.exporting
	s.mu.Unlock()
	return ex
}

func (s *Exporter) export(data *agent.ExportRequest, messageID string) {
	started := time.Now()

	handleError := func(err error) {
		s.logger.Error("export finished with error", "err", err)
		err2 := s.sendFailedEvent(data.JobID, started, time.Now(), err)
		if err2 != nil {
			s.logger.Error("error sending failed export event", "sending_err", err2, "export_err", err)
		}
	}

	if len(data.Integrations) == 0 {
		handleError(errors.New("passed export request has no integrations, ignoring it"))
		return
	}

	if err := s.sendStartExportEvent(data.JobID, data.Integrations); err != nil {
		handleError(fmt.Errorf("error sending export response start event: %v", err))
		return
	}

	exportResult, err := s.doExport(data, messageID)
	if err != nil {
		if _, o := err.(*subcommand.Cancelled); o {
			handleError(errors.New("export cancelled"))
			return
		}
		handleError(err)
		return
	}
	s.logger.Info("sending back export event")

	if data.UploadURL == nil || *data.UploadURL == "" {
		handleError(errors.New("No UploadURL provided in ExportRequest"))
		return
	}

	err = s.sendSuccessEvent(data.JobID, started, exportResult, *data.UploadURL, data.Integrations)
	if err != nil {
		s.logger.Error("error sending back export completed event", "err", err)
	}
}

type exportResult struct {
	UploadPartsCount int
	UploadFileSize   int64
	Integrations     []exportResultIntegration
}

type exportResultIntegration struct {
	IsIncremental bool
	Error         string
	Duration      time.Duration
	ProjectErrors []agent.ExportResponseIntegrationsProjectErrors
}

func (s *Exporter) doExport(data *agent.ExportRequest, messageID string) (res exportResult, rerr error) {
	isIncremental, partsCount, fileSize, res0, err := s.doExport2(data, messageID)
	if err != nil {
		rerr = err
		return
	}
	res.UploadPartsCount = partsCount
	res.UploadFileSize = fileSize
	for i, in0 := range res0.Integrations {
		in := exportResultIntegration{}
		in.IsIncremental = isIncremental[i]
		in.Error = in0.Error
		in.Duration = in0.Duration
		for _, pr0 := range in0.Projects {
			pr := agent.ExportResponseIntegrationsProjectErrors{}
			pr.ID = pr0.ID
			pr.RefID = pr0.RefID
			pr.ReadableID = pr0.ReadableID
			pr.Error = pr0.Error
			in.ProjectErrors = append(in.ProjectErrors, pr)
		}
		res.Integrations = append(res.Integrations, in)
	}
	return
}

func (s *Exporter) doExport2(data *agent.ExportRequest, messageID string) (isIncremental []bool, partsCount int, fileSize int64, res cmdexport.Result, rerr error) {
	s.logger.Info("processing export request", "job_id", data.JobID, "request_date", data.RequestDate.Rfc3339, "reprocess_historical", data.ReprocessHistorical)

	err := s.backupRestoreStateDir()
	if err != nil {
		rerr = fmt.Errorf("could not manage backup dir for export: %v", err)
		return
	}

	var integrations []cmdexport.Integration

	// add in additional integrations defined in config
	// TODO: not currently used, remove or disable
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
		s.logger.Info("exporting integration", "name", integration.Name, "len(exclusions)", len(integration.Exclusions), "len(inclusions)", len(integration.Inclusions))
		conf, err := inconfig.ConfigFromEvent(integration.ToMap(), inconfig.IntegrationType(integration.SystemType), s.opts.PPEncryptionKey)
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

	logFile := ""
	res, logFile, err = s.execExport(integrations, data.ReprocessHistorical, messageID, data.JobID)
	if logFile != "" {
		defer os.Remove(logFile)
	}
	if err != nil {
		rerr = err
		return
	}

	s.logger.Info("export finished, running upload")

	partsCount, fileSize, err = cmdupload.Run(context.Background(), s.logger, s.opts.PinpointRoot, *data.UploadURL, data.JobID, s.conf.APIKey, logFile)
	if err != nil {
		if err == cmdupload.ErrNoFilesFound {
			s.logger.Info("skipping upload, no files generated")
			// do not return errors when no files to upload, which is ok for incremental
		} else {
			rerr = err
			return
		}
	}

	err = s.deleteBackupStateDir()
	if err != nil {
		rerr = err
		return
	}

	return
}

func (s *Exporter) getLastProcessed(lastProcessed *jsonstore.Store, in cmdexport.Integration) (string, error) {
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

func (s *Exporter) execExport(integrations []cmdexport.Integration, reprocessHistorical bool, messageID string, jobID string) (res cmdexport.Result, logFile string, rerr error) {

	agentConfig := s.opts.AgentConfig
	agentConfig.Backend.ExportJobID = jobID

	c, err := subcommand.New(subcommand.Opts{
		Logger:            s.logger,
		Tmpdir:            s.opts.FSConf.Temp,
		IntegrationConfig: agentConfig,
		AgentConfig:       s.conf,
		Integrations:      integrations,
		DeviceInfo:        s.deviceInfo,
	})
	if err != nil {
		rerr = err
		return
	}
	args := []string{
		"--log-level", logutils.LogLevelToString(s.opts.LogLevelSubcommands),
	}
	if reprocessHistorical {
		args = append(args, "--reprocess-historical=true")
	}
	logFile, rerr = c.RunKeepLogFile(context.Background(), "export", messageID, &res, args...)
	//s.logger.Debug("executed export command, got res", "v", fmt.Sprintf("%v", res))

	return
}
