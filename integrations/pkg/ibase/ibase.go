package ibase

import (
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent/rpcdef"
)

func MainFunc(construct func(logger hclog.Logger) rpcdef.Integration) {

	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Output:     os.Stderr,
		JSONFormat: true,
	})
	impl := construct(logger)
	var pluginMap = map[string]plugin.Plugin{
		"integration": &rpcdef.IntegrationPlugin{Impl: impl},
	}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
