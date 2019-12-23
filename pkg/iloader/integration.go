package iloader

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"

	"github.com/mitchellh/go-homedir"

	"github.com/pinpt/go-common/fileutil"

	"path/filepath"
	"strings"

	"github.com/pinpt/agent/pkg/build"
	"github.com/pinpt/agent/pkg/integrationid"

	"github.com/pinpt/agent/pkg/fsconf"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent/rpcdef"
)

type IntegrationOpts struct {
	Logger                     hclog.Logger
	Agent                      rpcdef.Agent
	ID                         integrationid.ID
	Locs                       fsconf.Locs
	IntegrationsDir            string
	DevUseCompiledIntegrations bool
}

type Integration struct {
	ID integrationid.ID

	opts   IntegrationOpts
	logger hclog.Logger

	logFileLoc string
	logFile    *os.File

	pluginClient     *plugin.Client
	rpcClientGeneric plugin.ClientProtocol
	rpcClient        rpcdef.Integration

	closed bool
}

func NewIntegration(opts IntegrationOpts) (*Integration, error) {
	if opts.Logger == nil || opts.Agent == nil || opts.ID.Empty() || opts.Locs.Root == "" || opts.IntegrationsDir == "" {
		panic("provide all opts")
	}
	s := &Integration{}
	s.ID = opts.ID
	s.opts = opts
	s.logger = opts.Logger.With("intg", s.opts.ID.String())
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

func (s *Integration) LogFile() string {
	return s.logFile.Name()
}

func (s *Integration) RPCClient() rpcdef.Integration {
	return s.rpcClient
}

func prodIntegrationCommand(integrationsDir string, integrationName string) (*exec.Cmd, error) {
	binName := integrationName
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binSubDir := filepath.Join(integrationsDir, "bin")

	// updater downloads integrations into bin subdir, to allow renaming old to new dir
	// if bin dir does not exist this is a manual install and integrations are provided directly in root
	if fileutil.FileExists(binSubDir) {
		integrationsDir = filepath.Join(integrationsDir, "bin")
	}

	bin := filepath.Join(integrationsDir, binName)
	if !fileutil.FileExists(bin) {
		return nil, fmt.Errorf("integration binary not found: %v path: %v", integrationName, bin)
	}
	return exec.Command(bin), nil
}

func devIntegrationCommand(binaryName string) (*exec.Cmd, error) {
	gop := os.Getenv("GOPATH")
	if gop == "" {
		home, err := homedir.Dir()
		if err != nil {
			return nil, fmt.Errorf("could not get default gopath: %v", err)
		}
		gop = filepath.Join(home, "go")
	}
	integrationsDir := filepath.Join(gop, "src", "github.com/pinpt/agent/integrations")
	integrationDir := filepath.Join(integrationsDir, binaryName)
	if !fileutil.FileExists(integrationDir) {
		return nil, fmt.Errorf("integration package not found: %v dir: %v", binaryName, integrationDir)
	}

	packageName := "github.com/pinpt/agent/integrations/" + binaryName

	// build to catch compile errors
	// we don't need the resulting binary
	cmd := exec.Command("go", "build", "-o", filepath.Join(os.TempDir(), "out"), packageName)

	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return exec.Command("go", "run", packageName), nil
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
	dir := s.opts.Locs.LogsIntegrations
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	s.logFileLoc = filepath.Join(dir, s.ID.String())
	f, err := os.Create(s.logFileLoc)
	if err != nil {
		return err
	}
	s.logFile = f
	return nil
}

func (s *Integration) setupRPC() error {
	var cmd *exec.Cmd
	if build.IsProduction() || s.opts.DevUseCompiledIntegrations {
		var err error
		cmd, err = prodIntegrationCommand(s.opts.IntegrationsDir, s.ID.Name)
		if err != nil {
			return err
		}
	} else {
		var err error
		cmd, err = devIntegrationCommand(s.ID.Name)
		if err != nil {
			return err
		}
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		Stderr:          s.logFile,
		Logger:          s.logger,
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         rpcdef.PluginMap,
		Cmd:             cmd,
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC},
		Managed: true,
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
