package cmdservicerunnorestarts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/pinpt/agent/cmd/cmdservicerunnorestarts/updater"
	"github.com/pinpt/agent/pkg/build"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *runner) handleUpdateEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for update requests")

	errorsChan := make(chan error, 1)

	actionConfig := action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   agent.UpdateRequestTopic.String(),
		Errors:  errorsChan,
		Headers: map[string]string{
			"customer_id": s.conf.CustomerID,
			"uuid":        s.conf.DeviceID,
		},
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.UpdateRequest)
		//headers, err := parseHeader(instance.Message().Headers)
		//if err != nil {
		//	return nil, fmt.Errorf("error parsing header. err %v", err)
		//}

		version := req.ToVersion

		s.logger.Info("received update request", "version", version)

		sendEvent := func(resp *agent.UpdateResponse) (datamodel.ModelSendEvent, error) {
			resp.RequestID = req.ID
			resp.UUID = s.conf.DeviceID
			date.ConvertToModel(time.Now(), &resp.EventDate)
			s.deviceInfo.AppendCommonInfo(resp)
			return datamodel.NewModelSendEvent(resp), nil
		}

		oldVersion, err := s.updateTo(version)
		if err != nil {
			s.logger.Error("Update failed", "err", err)
			resp := &agent.UpdateResponse{}
			resp.Error = pstrings.Pointer(err.Error())
			s.deviceInfo.AppendCommonInfo(resp)
			return sendEvent(resp)
		}

		s.logger.Info("Update completed", "from_version", oldVersion, "to_version", version)

		go func() {
			if oldVersion == version {
				s.logger.Info("Same version as before, nothing to do")
				return
			}
			s.logger.Info("Restarting in 10s")
			time.Sleep(10 * time.Second)
			os.Exit(1)
		}()

		resp := &agent.UpdateResponse{}
		resp.FromVersion = oldVersion
		resp.ToVersion = version
		return sendEvent(resp)
	}

	go func() {
		for err := range errorsChan {
			s.logger.Error("error in integration requests", "err", err)
		}
	}()

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}

	sub.WaitForReady()

	return func() { sub.Close() }, nil
}

func (s *runner) updateTo(version string) (oldVersion string, rerr error) {
	if !build.IsProduction() {
		rerr = errors.New("Automatic update is only supported for production builds")
		return
	}

	switch runtime.GOOS {
	case "darwin":
		rerr = errors.New("Automatic update is not supported on macOS")
		return
	case "linux", "windows":
	default:
		rerr = errors.New("Automatic update is not supported on: " + runtime.GOOS)
		return
	}

	if version == "" {
		rerr = errors.New("Can't update. Empty version provided.")
		return
	}

	err := build.ValidateVersion(version)
	if err != nil {
		rerr = fmt.Errorf("Can't update invalid version format provided: %v", err)
		return
	}

	oldVersion = os.Getenv("PP_AGENT_VERSION")
	if oldVersion == "" {
		rerr = fmt.Errorf("Can't update, could not retrieve current version")
		return
	}

	if version == oldVersion {
		s.logger.Info("Skipping requested update, already at the target version", "v", version)
		return
	}

	// when updating using PP_AGENT_UPDATE_VERSION exporter is not set yet and no onboarding or exporting is happening
	if s.exporter != nil {
		status := s.getPing()
		if status.Onboarding {
			rerr = fmt.Errorf("Can't update, onboarding is in progress")
			return
		}
		if status.Exporting {
			rerr = fmt.Errorf("Can't update, exporting is in progress")
			return
		}
	}

	upd := updater.New(s.logger, s.fsconf, s.conf)
	err = upd.Update(version)
	if err != nil {
		rerr = fmt.Errorf("Could not update: %v", err)
		return
	}

	return
}
