package cmdexport

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/event"
)

func (s *export) sendProgress(ctx context.Context, progressData []byte) error {
	jobID := s.Opts.AgentConfig.Backend.ExportJobID
	if jobID == "" {
		return errors.New("ExportJobID is not specified in config")
	}
	b, err := json.Marshal(progressData)
	if err != nil {
		return err
	}
	if len(b) > 10*1024*1024 {
		return errors.New("progress data is >10MB skipping send")
	}
	str := string(progressData)
	data := &agent.ExportResponse{
		JobID:   jobID,
		RefType: "progress",
		Type:    agent.ExportResponseTypeExport,
		Data:    &str,
		Success: true,
	}
	s.deviceInfo.AppendCommonInfo(data)
	publishEvent := event.PublishEvent{
		Object: data,
		Headers: map[string]string{
			"uuid": s.EnrollConf.DeviceID,
		},
	}

	return event.Publish(ctx, publishEvent, s.EnrollConf.Channel, s.EnrollConf.APIKey)
}
