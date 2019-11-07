package cmdservicerunnorestarts

import (
	"context"
	"encoding/json"

	"github.com/pinpt/agent.next/cmd/cmdvalidateconfig"
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

func (s *runner) validate(ctx context.Context, name string, systemType IntegrationType, config map[string]interface{}) (res cmdvalidateconfig.Result, _ error) {
	s.logger.Info("validating config for integration", "name", name)
	// convert to non pointer strings
	config, err := depointer(config)
	if err != nil {
		return res, err
	}
	inConf, agentIn, err := convertConfig(name, systemType, config, []string{})
	if err != nil {
		return res, err
	}
	in := cmdvalidateconfig.Integration{}
	in.Name = agentIn.Name
	in.Type = agentIn.Type
	in.Config = inConf

	integrations := []cmdvalidateconfig.Integration{in}

	c := &subCommand{
		ctx:          ctx,
		logger:       s.logger,
		tmpdir:       s.fsconf.Temp,
		config:       s.agentConfig,
		conf:         s.conf,
		integrations: integrations,
		deviceInfo:   s.deviceInfo,
	}
	c.validate()

	err = c.run("validate-config", &res)
	if err != nil {
		return res, err
	}
	s.logger.Info("validation completed", "success", res.Success)
	if !res.Success {
		s.logger.Info("validation failed", "err", res.Errors)
	}
	return res, nil
}
