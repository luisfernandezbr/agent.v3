package cmdservicerun

import (
	"context"

	"github.com/pinpt/agent.next/cmd/cmdexportonboarddata"
	"github.com/pinpt/agent.next/cmd/cmdintegration"

	"github.com/pinpt/agent.next/cmd/cmdvalidateconfig"
)

func (s *runner) getOnboardData(ctx context.Context, config cmdintegration.Integration, objectType string) (res cmdexportonboarddata.Result, _ error) {
	s.logger.Info("getting onboarding data for integration", "name", config.Name, "objectType", objectType)

	integrations := []cmdvalidateconfig.Integration{config}

	args := []string{"export-onboard-data", "--object-type", objectType}

	fs, err := newFsPassedParams(s.fsconf.Temp, []kv{
		{"--agent-config-file", s.agentConfig},
		{"--integrations-file", integrations},
	})
	if err != nil {
		return res, err
	}
	args = append(args, fs.Args()...)
	defer fs.Clean()

	err = s.runCommand(ctx, &res, args)

	//s.logger.Debug("got onboard data", "res", pjson.Stringify(res))

	s.logger.Info("getting onboard data completed", "success", res.Success)

	if err != nil {
		return res, err
	}

	return res, nil
}
