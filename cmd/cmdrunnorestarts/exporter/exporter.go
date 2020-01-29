// Package exporter for scheduling and executing exports as part of run command
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
	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/go-common/event"

	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/deviceinfo"
	"github.com/pinpt/agent/pkg/fs"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/jsonstore"
	"github.com/pinpt/agent/pkg/logutils"
	"github.com/pinpt/agent/pkg/structmarshal"

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

// Run starts processing ExportQueue. This is a blocking call.
func (s *Exporter) Run() {
	go func() {
		for req := range s.queueRequestForwarder {
			req2 := Request{}
			err := structmarshal.MapToStruct(req.Data, &req2)
			if err != nil {
				s.logger.Error("could not unmarshal export request from map", "err", err)
			}
			s.setRunning(true)
			s.export(req2.Data, req2.MessageID)
			s.setRunning(false)
			req.Done <- struct{}{}
		}
	}()

	go func() {
		err := s.queue.Run(context.Background())
		if err != nil {
			panic(err)
		}
	}()

	for req := range s.ExportQueue {
		data := req.Data

		handleError := func(err error) {
			s.logger.Error("export finished with error", "err", err)
			err2 := s.sendExportFailedEvent(data.JobID, time.Now(), time.Now(), err)
			if err2 != nil {
				s.logger.Error("error sending failed export event", "sending_err", err2, "export_err", err)
			}
		}

		// have handling of request deadline before we save the request on disk, otherwise requests would not be retried if failed
		// need deadline here to prevent a bunch of older requests from queue being accepted
		requestDate := datetime.DateFromEpoch(data.RequestDate.Epoch)
		const exportEventDeadline = 5 * time.Minute
		if requestDate.Before(time.Now().Add(-exportEventDeadline)) {
			handleError(fmt.Errorf("export request date is older than deadline, ignoring. deadline: %v", exportEventDeadline.String()))
			continue
		}

		m, err := structmarshal.StructToMap(req)
		if err != nil {
			handleError(fmt.Errorf("could not marshal export request to map: %v", err))
			continue
		}

		s.queue.Input <- m
	}
	return
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
func (s *Exporter) sendExportEvent(jobID string, data agent.ExportResponse) error {
	data.JobID = jobID
	data.RefType = "export"
	data.Type = agent.ExportResponseTypeExport

	datap := &data
	s.deviceInfo.AppendCommonInfo(datap)
	publishEvent := event.PublishEvent{
		Object: datap,
		Headers: map[string]string{
			"uuid": s.conf.DeviceID,
		},
	}
	return event.Publish(context.Background(), publishEvent, s.conf.Channel, s.conf.APIKey)
}

func (s *Exporter) sendExportEventSettingIntegrations(jobID string, data agent.ExportResponse, ints []agent.ExportRequestIntegrations, isIncremental []bool) error {
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
	return s.sendExportEvent(jobID, data)
}

func (s *Exporter) sendStartExportEvent(jobID string, ints []agent.ExportRequestIntegrations) error {
	data := agent.ExportResponse{
		State:   agent.ExportResponseStateStarting,
		Success: true,
	}
	return s.sendExportEventSettingIntegrations(jobID, data, ints, nil)
}

func (s *Exporter) sendExportFailedEvent(jobID string, started, ended time.Time, err error) error {
	s.logger.Info("sending ExportResponse Completed Success=false")
	data := agent.ExportResponse{
		State: agent.ExportResponseStateCompleted,
	}
	date.ConvertToModel(started, &data.StartDate)
	date.ConvertToModel(ended, &data.EndDate)
	errstr := err.Error()
	data.Error = &errstr
	data.Success = false
	err = s.sendExportEvent(jobID, data)
	if err != nil {
		return err
	}
	s.logger.Info("sent ExportResponse Completed Success=false")
	return nil
}

func (s *Exporter) sendEndExportEvent(jobID string, started, ended time.Time, partsCount int, filesize int64, uploadurl *string, ints []agent.ExportRequestIntegrations, isIncremental []bool) error {
	s.logger.Info("sending ExportResponse Completed Success=true")

	data := agent.ExportResponse{
		State:           agent.ExportResponseStateCompleted,
		Size:            filesize,
		UploadURL:       uploadurl,
		UploadPartCount: int64(partsCount),
	}
	date.ConvertToModel(started, &data.StartDate)
	date.ConvertToModel(ended, &data.EndDate)
	data.Success = true
	err := s.sendExportEventSettingIntegrations(jobID, data, ints, isIncremental)
	if err != nil {
		return err
	}
	s.logger.Info("sent ExportResponse Completed Success=true")
	return nil
}

func (s *Exporter) export(data *agent.ExportRequest, messageID string) {
	started := time.Now()

	handleError := func(err error) {
		s.logger.Error("export finished with error", "err", err)
		err2 := s.sendExportFailedEvent(data.JobID, started, time.Now(), err)
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

	isIncremental, partsCount, fileSize, err := s.doExport(data, messageID)
	if err != nil {
		if _, o := err.(*subcommand.Cancelled); o {
			handleError(errors.New("export cancelled"))
			return
		}
		handleError(err)
		return
	}
	s.logger.Info("sending back export event")
	err = s.sendEndExportEvent(data.JobID, started, time.Now(), partsCount, fileSize, data.UploadURL, data.Integrations, isIncremental)
	if err != nil {
		s.logger.Error("error sending back export completed event", "err", err)
	}
}

func (s *Exporter) doExport(data *agent.ExportRequest, messageID string) (isIncremental []bool, partsCount int, fileSize int64, rerr error) {
	s.logger.Info("processing export request", "job_id", data.JobID, "request_date", data.RequestDate.Rfc3339, "reprocess_historical", data.ReprocessHistorical)

	err := s.backupRestoreStateDir()
	if err != nil {
		rerr = fmt.Errorf("could not manage backup dir for export: %v", err)
		return
	}

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

	logFile, err := s.execExport(integrations, data.ReprocessHistorical, messageID, data.JobID)
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

func (s *Exporter) backupRestoreStateDir() error {
	locs := s.opts.FSConf

	stateExists, err := fs.Exists(locs.State)
	if err != nil {
		return err
	}
	if !stateExists {
		err = os.MkdirAll(locs.State, 0755)
		if err != nil {
			return fmt.Errorf("could not create dir to save state, err: %v", err)
		}
		return nil
	}

	backupExists, err := fs.Exists(locs.Backup)
	if err != nil {
		return err
	}

	if backupExists {

		s.logger.Info("previous export/upload did not finish since we found a backup dir, restoring previous state and trying again")

		// restore the backup, but also keep backup, so we could restore to it again

		if err := os.RemoveAll(locs.LastProcessedFile); err != nil {
			return err
		}

		if err := fs.CopyFile(locs.LastProcessedFileBackup, locs.LastProcessedFile); err != nil {
			// would happen when running first historical because backup does not have any state yet, but we should be able to recover to that in case of error
			if !os.IsNotExist(err) {
				return err
			}
		}

		if err := os.RemoveAll(locs.RipsrcCheckpoints); err != nil {
			return err
		}

		if err := fs.CopyDir(locs.RipsrcCheckpointsBackup, locs.RipsrcCheckpoints); err != nil {
			// would happen when running first historical because backup does not have any state yet, but we should be able to recover to that in case of error
			if !os.IsNotExist(err) {
				return err
			}
		}

		return nil
	}

	// save backup

	err = os.MkdirAll(locs.Backup, 0755)
	if err != nil {
		return err
	}

	if err := fs.CopyFile(locs.LastProcessedFile, locs.LastProcessedFileBackup); err != nil {
		// would happen if export did not create last processed file, which is very unlikely
		if !os.IsNotExist(err) {
			return err
		}
	}

	if err := fs.CopyDir(locs.RipsrcCheckpoints, locs.RipsrcCheckpointsBackup); err != nil {
		// would happen if export did not have any ripsrc data
		if !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func (s *Exporter) deleteBackupStateDir() error {
	locs := s.opts.FSConf
	if err := os.RemoveAll(locs.Backup); err != nil {
		return fmt.Errorf("error deleting export backup file: %v", err)
	}
	return nil
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

func (s *Exporter) execExport(integrations []cmdexport.Integration, reprocessHistorical bool, messageID string, jobID string) (logFile string, rerr error) {

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
	logFile, rerr = c.RunKeepLogFile(context.Background(), "export", messageID, nil, args...)

	return
}
