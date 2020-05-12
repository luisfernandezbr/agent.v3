package cmdrunnorestarts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pinpt/go-common/datetime"

	"github.com/pinpt/agent/integrations/pkg/mutate"

	"github.com/pinpt/agent/cmd/cmdmutate"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/subcommand"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
)

const ignoreMutationRequestsFromOperatorOlderThan = 15 * time.Second

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
		logger := s.logger.With("job_id", req.JobID, "action", req.Action, "in", req.IntegrationName, "webapp_request_date", req.WebappRequestDate.Rfc3339, "agent_request_sent_date", req.AgentRequestSentDate.Rfc3339)

		logger.Info("received mutation request")
		start := time.Now()

		setTiming := func(resp *agent.IntegrationMutationResponse) {
			resp.WebappRequestDate = agent.IntegrationMutationResponseWebappRequestDate(req.WebappRequestDate)
			resp.AgentRequestSentDate = agent.IntegrationMutationResponseAgentRequestSentDate(req.AgentRequestSentDate)
			date.ConvertToModel(start, &resp.AgentReceivedRequestDate)
			date.ConvertToModel(time.Now(), &resp.AgentResponseSentDate)
		}

		sendEvent := func(resp *agent.IntegrationMutationResponse) (datamodel.ModelSendEvent, error) {
			setTiming(resp)
			logger.Info("processed mutation req", "dur", time.Since(start).String())
			resp.JobID = req.JobID
			date.ConvertToModel(time.Now(), &resp.EventDate)
			s.deviceInfo.AppendCommonInfo(resp)
			return datamodel.NewModelSendEvent(resp), nil
		}

		sendError := func(errorCode string, err error) (datamodel.ModelSendEvent, error) {
			logger.Info("mutation failed", "err", err)
			resp := &agent.IntegrationMutationResponse{}
			errStr := err.Error()
			resp.Error = &errStr
			switch errorCode {
			case mutate.ErrNotFound:
				resp.ErrorCode = agent.IntegrationMutationResponseErrorCodeNotFound
			}
			return sendEvent(resp)
		}

		header, err := parseHeader(instance.Message().Headers)
		if err != nil {
			return sendError("", fmt.Errorf("error parsing header. err %v", err))
		}

		agentRequestSentDate := datetime.DateFromEpoch(req.AgentRequestSentDate.Epoch)
		age := time.Since(agentRequestSentDate)

		if age > ignoreMutationRequestsFromOperatorOlderThan {
			// since we process mutation requests single threaded, this can happen in case of higher concurrency
			// could also happen in case of event delivery issues
			return sendError("", fmt.Errorf("ignoring mutation request older than %v, actual: %v", ignoreMutationRequestsFromOperatorOlderThan, age))
		}

		conf := inconfig.IntegrationAgent{}
		//conf.ID  not setting id
		conf.Name = req.IntegrationName
		conf.Config.RefreshToken = req.Authorization.RefreshToken
		conf.Config.AccessToken = req.Authorization.AccessToken
		if req.Authorization.URL != nil {
			conf.Config.URL = *req.Authorization.URL
		}
		conf.Type = inconfig.IntegrationType(req.SystemType)
		err = inconfig.AdjustFields(&conf)
		if err != nil {
			return sendError("", fmt.Errorf("could not convert jira: %v", err))
		}

		var mutationData interface{}
		err = json.Unmarshal([]byte(req.Data), &mutationData)
		if err != nil {
			return sendError("", fmt.Errorf("mutation data is not valid json: %v", err))
		}

		mutation := cmdmutate.Mutation{}
		mutation.Fn = req.Action.String()
		mutation.Data = mutationData
		res, err := s.execMutate(context.Background(), conf, header.MessageID, mutation)
		if err != nil {
			return sendError("", err)
		}
		if res.Error != "" || res.ErrorCode != "" {
			return sendError(res.ErrorCode, errors.New(res.Error))
		}

		resp := &agent.IntegrationMutationResponse{}
		resp.Success = true

		mutatedObjectsJSON, err := json.Marshal(res.MutatedObjects)
		if err != nil {
			return sendError("", err)
		}
		resp.UpdatedObjects = string(mutatedObjectsJSON)

		webappResponseJSON, err := json.Marshal(res.WebappResponse)
		if err != nil {
			return sendError("", err)
		}
		resp.WebappResponse = string(webappResponseJSON)

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
