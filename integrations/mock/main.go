package main

import (
	"context"
	"os"

	"github.com/pinpt/agent2/rpcdef"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

type Integration struct {
	logger hclog.Logger
	agent  rpcdef.Agent
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	return nil
}

func (s *Integration) Export(ctx context.Context) error {
	s.logger.Info("mock.export called")

	objs := []rpcdef.ExportObj{
		{Data: 1},
		{Data: "a"},
	}

	s.agent.SendExported(objs)
	return nil
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	integration := &Integration{
		logger: logger,
	}

	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]plugin.Plugin{
		"integration": &rpcdef.IntegrationPlugin{Impl: integration},
	}

	logger.Info("example plugin log message")

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
