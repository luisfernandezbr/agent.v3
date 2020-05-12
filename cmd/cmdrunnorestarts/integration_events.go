package cmdrunnorestarts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/encrypt"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	pjson "github.com/pinpt/go-common/json"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *runner) handleIntegrationEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for integration requests")

	actionConfig := action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   agent.IntegrationRequestModelName.String(),
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

		integration2 := inconfig.IntegrationAgent{}
		integration2.Name = integration.Name
		integration2.Type = inconfig.IntegrationType(integration.SystemType)

		structmarshal.StructToStruct(integration.Authorization, &integration2.Config)

		if integration.Authorization.APIToken != nil {
			integration2.Config.APIKey = *integration.Authorization.APIToken
		}
		inconfig.AdjustFields(&integration2)

		res, err := s.validate(ctx, headers.MessageID, integration2)
		if err != nil {
			return rerr(fmt.Errorf("could not call validate, err: %v", err))
		}

		if !res.Success {
			return rerr(errors.New(strings.Join(res.Errors, ", ")))
		}

		encrAuthData, err := encrypt.EncryptString(pjson.Stringify(integration.Authorization), s.conf.PPEncryptionKey)
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

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}

	sub.WaitForReady()

	return func() { sub.Close() }, nil
}
