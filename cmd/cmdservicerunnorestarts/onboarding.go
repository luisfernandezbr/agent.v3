package cmdservicerunnorestarts

import (
	"context"

	"github.com/pinpt/agent.next/cmd/cmdexportonboarddata"
	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/pinpt/agent.next/cmd/cmdservicerunnorestarts/subcommand"

	"github.com/pinpt/agent.next/cmd/cmdvalidateconfig"
)

func (s *runner) getOnboardData(ctx context.Context, config cmdintegration.Integration, messageID string, objectType string) (res cmdexportonboarddata.Result, _ error) {
	s.logger.Info("getting onboarding data for integration", "name", config.Name, "objectType", objectType)

	integrations := []cmdvalidateconfig.Integration{config}

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

	err = c.Run(ctx, "export-onboard-data", messageID, &res, "--object-type", objectType)

	s.logger.Info("getting onboard data completed", "success", res.Success, "err", res.Error)
	if err != nil {
		return res, err
	}

	return res, nil
}
