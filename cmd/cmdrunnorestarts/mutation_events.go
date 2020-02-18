package cmdrunnorestarts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pinpt/agent/cmd/cmdmutate"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/subcommand"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
)

func (s *runner) handleMutationEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for mutation requests")

	actionConfig := action.Config{
		APIKey:  s.conf.APIKey,
		GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
		Channel: s.conf.Channel,
		Factory: factory,
		Topic:   agent.IntegrationMutationRequestModelName.String(),
		Headers: map[string]string{
			"customer_id": s.conf.CustomerID,
			"uuid":        s.conf.DeviceID,
		},
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.IntegrationMutationRequest)
		s.logger.Info("received mutation request", "id", req.ID)
		start := time.Now()
		sendEvent := func(resp *agent.IntegrationMutationResponse) (datamodel.ModelSendEvent, error) {
			s.logger.Info("processed mutation req", "dur", time.Since(start).String())

			//resp.RefType = integration.Name
			//resp.RefID = integration.RefID
			//resp.RequestID = req.ID
			date.ConvertToModel(time.Now(), &resp.EventDate)
			s.deviceInfo.AppendCommonInfo(resp)
			return datamodel.NewModelSendEvent(resp), nil
		}

		sendError := func(err error) (datamodel.ModelSendEvent, error) {
			s.logger.Info("mutation failed", "err", err)
			resp := &agent.IntegrationMutationResponse{}
			errStr := err.Error()
			resp.Error = &errStr
			return sendEvent(resp)
		}

		header, err := parseHeader(instance.Message().Headers)
		if err != nil {
			return sendError(fmt.Errorf("error parsing header. err %v", err))
		}

		data := map[string]interface{}{
			"name": req.IntegrationName,
			"authorization": map[string]interface{}{
				"authorization": req.Authorization.RefreshToken,
			},
		}

		conf, err := inconfig.AuthFromEvent(data, s.conf.PPEncryptionKey)
		conf.Type = inconfig.IntegrationType(req.SystemType)
		if err != nil {
			return sendError(err)
		}

		var mutationData interface{}
		err = json.Unmarshal([]byte(req.Data), &mutationData)
		if err != nil {
			return sendError(fmt.Errorf("mutation data is not valid json: %v", err))
		}

		mutation := cmdmutate.Mutation{}
		mutation.Fn = req.Action.String()
		mutation.Data = mutationData
		res, err := s.execMutate(context.Background(), conf, header.MessageID, mutation)
		if err != nil {
			return sendError(err)
		}
		if res.Error != "" {
			return sendError(errors.New(res.Error))
		}
		resp := &agent.IntegrationMutationResponse{}
		resp.Success = true
		objects, err := json.Marshal(res.Objects)
		if err != nil {
			return sendError(err)
		}
		resp.UpdatedObjects = string(objects)
		return sendEvent(resp)
	}

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}

	sub.WaitForReady()

	return func() { sub.Close() }, nil

}

func (s *runner) execMutate(ctx context.Context, config inconfig.IntegrationAgent, messageID string, mutation cmdmutate.Mutation) (res cmdmutate.Result, _ error) {
	integrations := []inconfig.IntegrationAgent{config}

	c, err := subcommand.New(subcommand.Opts{
		Logger:            s.logger,
		Tmpdir:            s.fsconf.Temp,
		IntegrationConfig: s.agentConfig,
		AgentConfig:       s.conf,
		Integrations:      integrations,
		DeviceInfo:        s.deviceInfo,
	})

	if err != nil {
		return res, err
	}

	mutationJSON, err := json.Marshal(mutation)
	if err != nil {
		return res, err
	}

	s.logger.Debug("executing mutation", "integration", config.Name, "mutation", string(mutationJSON))

	err = c.Run(ctx, "mutate", messageID, &res, "--mutation", string(mutationJSON))

	s.logger.Debug("executing mutation", "success", res.Success, "err", res.Error)

	if err != nil {
		return res, err
	}

	return res, nil
}
