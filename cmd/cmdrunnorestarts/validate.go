package cmdrunnorestarts

import (
	"context"
	"encoding/json"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/subcommand"
	"github.com/pinpt/agent/cmd/cmdvalidateconfig"
)

func depointer(data map[string]interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var res map[string]interface{}
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *runner) validate(ctx context.Context, messageID string, integration inconfig.IntegrationAgent) (res cmdvalidateconfig.Result, _ error) {
	s.logger.Info("validating config for integration", "name", integration.Name)

	c, err := subcommand.New(subcommand.Opts{
		Logger:            s.logger,
		Tmpdir:            s.fsconf.Temp,
		IntegrationConfig: s.agentConfig,
		AgentConfig:       s.conf,
		Integrations:      []inconfig.IntegrationAgent{integration},
		DeviceInfo:        s.deviceInfo,
	})
	if err != nil {
		return res, err
	}

	err = c.Run(ctx, "validate-config", messageID, &res)
	if err != nil {
		return res, err
	}
	s.logger.Info("validation completed", "success", res.Success)
	if !res.Success {
		s.logger.Info("validation failed", "err", res.Errors)
	}
	return res, nil
}
