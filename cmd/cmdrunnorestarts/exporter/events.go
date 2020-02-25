package exporter

import (
	"context"
	"fmt"
	"time"

	"github.com/pinpt/agent/pkg/aevent"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *Exporter) sendStartExportEvent(jobID string, ints []agent.ExportRequestIntegrations) error {
	data := agent.ExportResponse{
		State:   agent.ExportResponseStateStarting,
		Success: true,
	}
	err := s.sendExportEvent(jobID, data)
	if err != nil {
		return err
	}
	return nil
}

func (s *Exporter) sendFailedEvent(jobID string, started, ended time.Time, err error) error {
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

func (s *Exporter) sendSuccessEvent(jobID string, started time.Time, res exportResult, uploadURL string, requestedInts []agent.ExportRequestIntegrations) error {
	s.logger.Info("sending ExportResponse Completed Success=true")
	//s.logger.Debug("exportResult", "v", fmt.Sprintf("%+v", res))

	data := agent.ExportResponse{
		State:           agent.ExportResponseStateCompleted,
		Size:            res.UploadFileSize,
		UploadURL:       &uploadURL,
		UploadPartCount: int64(res.UploadPartsCount),
	}
	date.ConvertToModel(started, &data.StartDate)
	date.ConvertToModel(time.Now(), &data.EndDate)
	data.Success = true

	if len(res.Integrations) == 0 {
		return fmt.Errorf("could not get export result for integration, requested %v integration, but no integrations in result", len(requestedInts))
	}
	if len(res.Integrations) != len(requestedInts) {
		return fmt.Errorf("number of integrations requests, and number of integrations in results does not match, requested: %v, got: %v", len(requestedInts), len(res.Integrations))
	}

	for i, reqIn := range requestedInts {
		v := agent.ExportResponseIntegrations{
			IntegrationID: reqIn.ID,
			Name:          reqIn.Name,
			SystemType:    agent.ExportResponseIntegrationsSystemType(reqIn.SystemType),
		}
		in := res.Integrations[i]
		if in.Incremental {
			v.ExportType = agent.ExportResponseIntegrationsExportTypeIncremental
		} else {
			v.ExportType = agent.ExportResponseIntegrationsExportTypeHistorical
		}
		v.Error = in.Error
		// TODO: pass duration back to server as well
		//v.Duration = in.Duration
		v.EntityErrors = in.EntityErrors
		data.Integrations = append(data.Integrations, v)
	}

	err := s.sendExportEvent(jobID, data)
	if err != nil {
		return err
	}

	s.logger.Info("sent ExportResponse Completed Success=true")
	return nil
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
	// wait longer for export events, since if those are missed, processing will not continue normally
	deadline := time.Now().Add(15 * time.Minute)
	return aevent.Publish(context.Background(), publishEvent, s.conf.Channel, s.conf.APIKey, event.WithDeadline(deadline))
}
