package cmdexport

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent2/rpcdef"
)

func devIntegrationCommand(integrationName string) *exec.Cmd {
	// build to catch compile errors
	// we don't need the resulting binary
	cmd := exec.Command("go", "build", "-o", filepath.Join(os.TempDir(), "out"), "github.com/pinpt/agent2/integrations/"+integrationName)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return exec.Command("go", "run", "github.com/pinpt/agent2/integrations/"+integrationName)
}

type integration struct {
	export  *export
	logger  hclog.Logger
	name    string
	logFile *os.File

	pluginClient     *plugin.Client
	rpcClientGeneric plugin.ClientProtocol
	rpcClient        rpcdef.Integration

	agentDelegate agentDelegate
}

func newIntegration(exp *export, name string) (*integration, error) {
	s := &integration{}
	s.export = exp
	s.logger = s.export.logger
	s.name = name
	err := s.setupLogFile()
	if err != nil {
		return nil, err
	}
	err = s.setupRPC()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *integration) Close() error {
	//err := s.rpcClientGeneric.Close()
	//if err != nil {
	//	return err
	//}
	s.pluginClient.Kill()
	err := s.logFile.Close()
	if err != nil {
		return err
	}
	return nil
}

func (s *integration) setupLogFile() error {
	dir := s.export.dirs.logs
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, s.name))
	if err != nil {
		return err
	}
	s.logFile = f
	return nil
}

func (s *integration) setupRPC() error {
	client := plugin.NewClient(&plugin.ClientConfig{
		Stderr:          s.logFile,
		Logger:          s.logger,
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         rpcdef.PluginMap,
		Cmd:             devIntegrationCommand(s.name),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC},
	})
	s.pluginClient = client

	rpcClientGeneric, err := client.Client()
	if err != nil {
		return err
	}
	s.rpcClientGeneric = rpcClientGeneric

	rpcClientIface, err := rpcClientGeneric.Dispense("integration")
	if err != nil {
		return err
	}

	s.rpcClient = rpcClientIface.(rpcdef.Integration)

	s.agentDelegate = agentDelegate{
		export: s.export,
	}
	s.rpcClient.Init(s.agentDelegate)
	return nil
}
