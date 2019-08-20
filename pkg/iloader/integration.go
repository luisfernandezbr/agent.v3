package iloader

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"

	"path/filepath"
	"strings"

	"github.com/pinpt/agent.next/pkg/build"

	"github.com/pinpt/agent.next/pkg/fsconf"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent.next/rpcdef"
)

type IntegrationOpts struct {
	Logger                     hclog.Logger
	Agent                      rpcdef.Agent
	Name                       string
	Locs                       fsconf.Locs
	DevUseCompiledIntegrations bool
}

type Integration struct {
	opts   IntegrationOpts
	logger hclog.Logger
	name   string

	logFileLoc string
	logFile    *os.File

	pluginClient     *plugin.Client
	rpcClientGeneric plugin.ClientProtocol
	rpcClient        rpcdef.Integration

	closed bool
}

func NewIntegration(opts IntegrationOpts) (*Integration, error) {
	if opts.Logger == nil || opts.Agent == nil || opts.Name == "" || opts.Locs.Root == "" {
		panic("provide all opts")
	}
	s := &Integration{}
	s.opts = opts
	s.logger = opts.Logger.With("integration", s.opts.Name)
	s.name = s.opts.Name
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

func (s *Integration) Name() string {
	return s.name
}

func (s *Integration) LogFile() string {
	return s.logFile.Name()
}

func (s *Integration) RPCClient() rpcdef.Integration {
	return s.rpcClient
}

func prodIntegrationCommand(fslocs fsconf.Locs, integrationName string) *exec.Cmd {
	binName := integrationName
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	bin := filepath.Join(fslocs.Integrations, binName)
	return exec.Command(bin)
}

func devIntegrationCommand(integrationName string) *exec.Cmd {
	// build to catch compile errors
	// we don't need the resulting binary
	cmd := exec.Command("go", "build", "-o", filepath.Join(os.TempDir(), "out"), "github.com/pinpt/agent.next/integrations/"+integrationName)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return exec.Command("go", "run", "github.com/pinpt/agent.next/integrations/"+integrationName)
}

func (s *Integration) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
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

func (s *Integration) setupLogFile() error {
	dir := s.opts.Locs.Logs
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	s.logFileLoc = filepath.Join(dir, s.name)
	f, err := os.Create(s.logFileLoc)
	if err != nil {
		return err
	}
	s.logFile = f
	return nil
}

func (s *Integration) setupRPC() error {
	var cmd *exec.Cmd
	if build.IsProd() || s.opts.DevUseCompiledIntegrations {
		cmd = prodIntegrationCommand(s.opts.Locs, s.name)
	} else {
		cmd = devIntegrationCommand(s.name)
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		Stderr:          s.logFile,
		Logger:          s.logger,
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         rpcdef.PluginMap,
		Cmd:             cmd,
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

	s.rpcClient.Init(s.opts.Agent)
	return nil
}

func (s *Integration) CloseAndDetectPanic() (panicOut string, rerr error) {
	rerr = s.Close()
	b, err := ioutil.ReadFile(s.logFileLoc)
	if err != nil {
		if rerr != nil {
			return "", err
		}
		return
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if startsWith(line, "panic:") {
			return strings.Join(lines[i:], "\n"), rerr
		}
	}
	return "", rerr
}

func startsWith(str, prefix string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(prefix) > len(str) {
		return false
	}
	return str[0:len(prefix)] == prefix
}
