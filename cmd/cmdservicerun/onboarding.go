package cmdservicerun

import (
	"context"

	"github.com/pinpt/agent.next/cmd/cmdexportonboarddata"
	"github.com/pinpt/agent.next/cmd/cmdintegration"

	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/agent.next/cmd/cmdvalidateconfig"
	pjson "github.com/pinpt/go-common/json"
)

func (s *runner) getOnboardData(ctx context.Context, config cmdintegration.Integration, objectType string) (res cmdexportonboarddata.Result, _ error) {
	s.logger.Info("getting onboarding data for integration", "name", config.Name, "objectType", objectType)

	agent := cmdexport.AgentConfig{}
	agent.CustomerID = s.conf.CustomerID
	agent.PinpointRoot = s.opts.PinpointRoot

	integrations := []cmdvalidateconfig.Integration{config}

	err := s.runCommand(ctx, &res, []string{"export-onboard-data", "--agent-config-json", pjson.Stringify(agent), "--integrations-json", pjson.Stringify(integrations), "--object-type", objectType})

	//s.logger.Debug("got onboard data", "res", pjson.Stringify(res))

	s.logger.Info("getting onboard data completed", "success", res.Success)

	if err != nil {
		return res, err
	}

	return res, nil
}
