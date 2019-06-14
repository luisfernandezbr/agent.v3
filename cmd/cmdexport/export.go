package cmdexport

import (
	"context"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent2/rpcdef"
)

type export struct {
	logger       hclog.Logger
	pluginClient *plugin.Client
	sessions     *sessions
	integration  rpcdef.Integration
}

func newExport(logger hclog.Logger) *export {
	s := &export{}
	s.logger = logger
	err := s.setupPlugins()
	if err != nil {
		panic(err)
	}
	s.sessions = newSessions()

	ctx := context.Background()
	err = s.integration.Export(ctx)
	if err != nil {
		panic(err)
	}
	return s
}

func (s export) Destroy() {
	defer s.pluginClient.Kill()
}

func (s *export) setupPlugins() error {
	client := plugin.NewClient(&plugin.ClientConfig{
		Logger:          s.logger,
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         rpcdef.PluginMap,
		Cmd:             devIntegrationCommand("mock"),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC},
	})
	s.pluginClient = client

	rpcClient, err := client.Client()
	if err != nil {
		return err
	}

	raw, err := rpcClient.Dispense("integration")
	if err != nil {
		return err
	}

	s.integration = raw.(rpcdef.Integration)

	delegate := agentDelegate{
		export: s,
	}

	s.integration.Init(delegate)
	return nil
}

type sessions struct {
	m      map[int]session
	lastID int
}

func newSessions() *sessions {
	s := &sessions{}
	s.m = map[int]session{}
	return s
}

func (s *sessions) new(modelType string) (sessionID string) {
	s.lastID++
	id := s.lastID
	s.m[id] = session{
		state:     "started",
		modelType: modelType,
	}
	return strconv.Itoa(id)
}

func (s *sessions) get(sessionID string) session {
	id, err := strconv.Atoi(sessionID)
	if err != nil {
		panic(err)
	}
	return s.m[id]
}

type session struct {
	state     string
	modelType string
}
