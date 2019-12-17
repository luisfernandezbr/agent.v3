package cmdservicerunnorestarts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pinpt/agent/cmd/cmdservicerunnorestarts/inconfig"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/encrypt"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	pjson "github.com/pinpt/go-common/json"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *runner) handleIntegrationEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for integration requests")

	errorsChan := make(chan error, 1)

	actionConfig := action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   agent.IntegrationRequestTopic.String(),
		Errors:  errorsChan,
		Headers: map[string]string{
			"customer_id": s.conf.CustomerID,
			"uuid":        s.conf.DeviceID,
		},
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.IntegrationRequest)
		headers, err := parseHeader(instance.Message().Headers)
		if err != nil {
			return nil, fmt.Errorf("error parsing header. err %v", err)
		}
		integration := req.Integration

		s.logger.Info("received integration request", "integration", integration.Name)

		s.logger.Info("sending back integration response")

		// TODO: add connection validation

		sendEvent := func(resp *agent.IntegrationResponse) (datamodel.ModelSendEvent, error) {
			s.deviceInfo.AppendCommonInfo(resp)
			return datamodel.NewModelSendEvent(resp), nil
		}

		resp := &agent.IntegrationResponse{}
		resp.RefType = integration.Name
		resp.RefID = integration.RefID
		resp.RequestID = req.ID

		resp.UUID = s.conf.DeviceID
		date.ConvertToModel(time.Now(), &resp.EventDate)

		rerr := func(err error) (datamodel.ModelSendEvent, error) {
			s.logger.Error("integration request failed", "err", err)
			// error for everything else
			resp.Type = agent.IntegrationResponseTypeIntegration
			resp.Error = pstrings.Pointer(err.Error())
			return sendEvent(resp)
		}

		auth := integration.Authorization.ToMap()

		res, err := s.validate(ctx, integration.Name, headers.MessageID, inconfig.IntegrationType(req.Integration.SystemType), auth)
		if err != nil {
			return rerr(fmt.Errorf("could not call validate, err: %v", err))
		}

		if !res.Success {
			return rerr(errors.New(strings.Join(res.Errors, ", ")))
		}

		encrAuthData, err := encrypt.EncryptString(pjson.Stringify(auth), s.conf.PPEncryptionKey)
		if err != nil {
			return rerr(err)
		}

		resp.Message = "Success. Integration validated."
		resp.Success = true
		resp.Type = agent.IntegrationResponseTypeIntegration
		resp.Authorization = encrAuthData
		resp.ServerVersion = pstrings.Pointer(res.ServerVersion)
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
