package cmdrunnorestarts

/*
import "context"
func (s *runner) handleWebhookEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for webhook requests")
	return nil, nil
}
*/
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pinpt/agent/cmd/cmdmutate"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/subcommand"
	"github.com/pinpt/agent/cmd/cmdwebhook"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/v10/datamodel"
	"github.com/pinpt/go-common/v10/datetime"
	"github.com/pinpt/go-common/v10/event"
	"github.com/pinpt/go-common/v10/event/action"
)

// will pick those changes in incremental export instead
const ignoreWebhookRequestsOlderThan = 5 * time.Minute

func (s *runner) handleWebhookEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for webhook requests")

	actionConfig := action.Config{
		Subscription: event.Subscription{
			APIKey:  s.conf.APIKey,
			GroupID: fmt.Sprintf("agent-%v", s.conf.DeviceID),
			Channel: s.conf.Channel,
			Headers: map[string]string{
				"customer_id": s.conf.CustomerID,
				"uuid":        s.conf.DeviceID,
			},
			DisablePing: true,
		},
		Factory: factory,
		Topic:   agent.WebhookRequestModelName.String(),
	}

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		req := instance.Object().(*agent.WebhookRequest)
		integrationName := "github" // TODO: req.IntegrationName
		logger := s.logger.With("in", integrationName)

		start := time.Now()
		logger.Info("received webhook event")

		setTiming := func(resp *agent.WebhookResponse) {
			resp.EventAPIReceivedDate = agent.WebhookResponseEventAPIReceivedDate(req.EventAPIReceivedDate)
			resp.OperatorReceivedDate = agent.WebhookResponseOperatorReceivedDate(req.OperatorReceivedDate)
			date.ConvertToModel(start, &resp.AgentReceivedDate)
			date.ConvertToModel(time.Now(), &resp.AgentResponseSentDate)
		}

		sendEvent := func(resp *agent.WebhookResponse) (datamodel.ModelSendEvent, error) {
			setTiming(resp)
			logger.Info("processed webhook req", "dur", time.Since(start).String())
			resp.JobID = req.JobID
			date.ConvertToModel(time.Now(), &resp.EventDate)
			s.deviceInfo.AppendCommonInfo(resp)
			return datamodel.NewModelSendEvent(resp), nil
		}

		sendError := func(errorCode string, err error) (datamodel.ModelSendEvent, error) {
			logger.Info("webhook failed", "err", err)
			resp := &agent.WebhookResponse{}
			errStr := err.Error()
			resp.Error = &errStr
			return sendEvent(resp)
		}

		header, err := parseHeader(instance.Message().Headers)
		if err != nil {
			return sendError("", fmt.Errorf("error parsing header. err %v", err))
		}

		agentRequestSentDate := datetime.DateFromEpoch(req.EventAPIReceivedDate.Epoch)
		age := time.Since(agentRequestSentDate)

		if age > ignoreWebhookRequestsOlderThan {
			logger.Warn(fmt.Sprintf("ignoring webhook request older than %v, actual: %v", ignoreWebhookRequestsOlderThan, age))
			return nil, nil
		}

		conf, err := inconfig.AuthFromEvent(map[string]interface{}{
			"id":   "id1",
			"name": integrationName,
			"authorization": map[string]interface{}{
				"authorization": req.EncryptedAuthorization,
			},
		}, s.conf.PPEncryptionKey)
		if err != nil {
			return sendError("", fmt.Errorf("invalid webhook request: %v", err))
		}
		conf.Type = inconfig.IntegrationType(req.SystemType)

		webhookData := cmdwebhook.Data{}
		webhookData.Headers = req.Headers
		err = json.Unmarshal([]byte(req.Data), &webhookData.Body)
		if err != nil {
			return sendError("", fmt.Errorf("webhook data is not valid json: %v", err))
		}

		res, err := s.execWebhook(context.Background(), conf, header.MessageID, webhookData)
		if err != nil {
			return sendError("", err)
		}
		if res.Error != "" || res.ErrorCode != "" {
			return sendError(res.ErrorCode, errors.New(res.Error))
		}

		resp := &agent.WebhookResponse{}
		resp.Success = true

		mutatedObjectsJSON, err := json.Marshal(res.MutatedObjects)
		if err != nil {
			return sendError("", err)
		}
		resp.UpdatedObjects = string(mutatedObjectsJSON)

		return sendEvent(resp)
	}

	sub, err := action.Register(ctx, action.NewAction(cb), actionConfig)
	if err != nil {
		return nil, err
	}

	sub.WaitForReady()

	return func() { sub.Close() }, nil

}

func (s *runner) execWebhook(ctx context.Context, config inconfig.IntegrationAgent, messageID string, data cmdwebhook.Data) (res cmdmutate.Result, _ error) {
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

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return res, err
	}

	s.logger.Debug("executing webhook", "integration", config.Name, "data", string(dataJSON))

	err = c.Run(ctx, "webhook", messageID, &res, "--data", string(dataJSON))

	s.logger.Debug("executing webhook", "success", res.Success, "err", res.Error)

	if err != nil {
		return res, err
	}

	return res, nil
}
