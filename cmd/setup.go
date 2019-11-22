package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/pinpt/agent.next/cmd/pkg/cmdlogger"

	"github.com/pinpt/agent.next/pkg/filelog"
	"github.com/pinpt/agent.next/pkg/fsconf"

	"github.com/fatih/color"
	ps "github.com/mitchellh/go-ps"
	"github.com/spf13/cobra"
)

var (
	// set by makefile
	Version                = "dev"
	Commit                 = "head"
	IntegrationBinariesAll = ""
)

func Execute() {
	// using version in metrics and downloading integrations
	os.Setenv("PP_AGENT_VERSION", Version)
	// using commit as extra debug info
	os.Setenv("PP_AGENT_COMMIT", Commit)

	{
		// list of integrations for download
		// allow setting up from commandline for development use
		v := os.Getenv("PP_INTEGRATION_BINARIES_ALL")
		if v == "" {
			os.Setenv("PP_INTEGRATION_BINARIES_ALL", IntegrationBinariesAll)
		}
	}

	cmdRoot.Execute()
}

func getBannerColor() *color.Color {
	if runtime.GOOS == "windows" {
		p, _ := ps.FindProcess(os.Getppid())
		if p != nil && strings.Contains(p.Executable(), "powershell") {
			// colorize for powershell to make it more prominent
			return color.New(color.FgCyan)
		}
		// since we're in cmd it's usually black so make it more colorful
	}
	return color.New(color.FgHiBlue)
}

var cmdRoot = &cobra.Command{
	Use: "pinpoint-agent",
	Long: getBannerColor().Sprint(`    ____  _                   _       __ 
   / __ \(_)___  ____  ____  (_)___  / /_
  / /_/ / / __ \/ __ \/ __ \/ / __ \/ __/
 / ____/ / / / / /_/ / /_/ / / / / / /_  
/_/   /_/_/ /_/ .___/\____/_/_/ /_/\__/  
             /_/                         

	https://pinpoint.com
`),
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func exitWithErr(logger hclog.Logger, err error) {
	logger.Error("error: " + err.Error())
	os.Exit(1)
}

func getPinpointRoot(cmd *cobra.Command) (root string, err error) {
	root, _ = cmd.Flags().GetString("pinpoint-root")
	if root != "" {
		return root, nil
	}
	root, err = fsconf.DefaultRoot()
	if err != nil {
		return root, err
	}
	return root, nil
}

func pinpointLogWriter(pinpointRoot string) (io.Writer, error) {
	if pinpointRoot == "" {
		var err error
		pinpointRoot, err = fsconf.DefaultRoot()
		if err != nil {
			return nil, err
		}
	}
	fsloc := fsconf.New(pinpointRoot)
	if len(os.Args) <= 1 {
		return nil, fmt.Errorf("could not create log file, len(os.Args) <= 1, and we use subcommand as name")
	}
	logFile := filepath.Join(fsloc.Logs, os.Args[1])
	wr, err := filelog.NewSyncWriter(logFile)
	return wr, err
}

var insideDocker = isInsideDocker()

func flagPinpointRoot(cmd *cobra.Command) {
	var def string
	if insideDocker {
		def = "/etc/pinpoint"
	}
	cmd.Flags().String("pinpoint-root", def, "Custom location of pinpoint work dir.")
}

func flagsLogger(cmd *cobra.Command) {
	cmd.Flags().String("log-format", "", "Set to json to see log output in json")
	cmd.Flags().String("log-level", "debug", "Log level (debug or info)")
}

func integrationCommandFlags(cmd *cobra.Command) {
	flagsLogger(cmd)
	flagPinpointRoot(cmd)
	cmd.Flags().String("agent-config-json", "", "Agent config as json")
	cmd.Flags().String("agent-config-file", "", "Agent config json as file")
	cmd.Flags().String("integrations-json", "", "Integrations config as json")
	cmd.Flags().String("integrations-file", "", "Integrations config json as file")
	cmd.Flags().String("integrations-dir", defaultIntegrationsDir(), "Integrations dir")
}

func defaultIntegrationsDir() string {
	if insideDocker {
		return "/bin/integrations"
	}
	return ""
}

func integrationCommandOpts(cmd *cobra.Command) (hclog.Logger, cmdintegration.Opts) {
	logger := cmdlogger.NewLogger(cmd)

	opts := cmdintegration.Opts{}

	agentConfigFile, _ := cmd.Flags().GetString("agent-config-file")
	if agentConfigFile != "" {
		b, err := ioutil.ReadFile(agentConfigFile)
		if err != nil {
			exitWithErr(logger, fmt.Errorf("agent-config-file does not point to a correct file, err %v", err))
		}
		err = json.Unmarshal(b, &opts.AgentConfig)
		if err != nil {
			exitWithErr(logger, fmt.Errorf("agent-config-file contains invalid json: %v", err))
		}
	}

	agentConfigJSON, _ := cmd.Flags().GetString("agent-config-json")
	if agentConfigJSON != "" {
		err := json.Unmarshal([]byte(agentConfigJSON), &opts.AgentConfig)
		if err != nil {
			exitWithErr(logger, fmt.Errorf("agent-config-json is not valid: %v", err))
		}
	}

	// allow setting pinpoint root in either json or command line flag
	{
		v, _ := cmd.Flags().GetString("pinpoint-root")
		if v != "" {
			opts.AgentConfig.PinpointRoot = v
		}
	}

	// allow setting integrations-dir in both json and command line flag
	{
		v, _ := cmd.Flags().GetString("integrations-dir")
		if v != "" {
			opts.AgentConfig.IntegrationsDir = v
		}
	}

	integrationsFile, _ := cmd.Flags().GetString("integrations-file")
	if integrationsFile != "" {
		b, err := ioutil.ReadFile(integrationsFile)
		if err != nil {
			exitWithErr(logger, fmt.Errorf("integrations-file does not point to a correct file, err %v", err))
		}
		err = json.Unmarshal(b, &opts.Integrations)
		if err != nil {
			exitWithErr(logger, fmt.Errorf("integrations-file contains invalid json: %v", err))
		}
	}

	integrationsJSON, _ := cmd.Flags().GetString("integrations-json")
	if integrationsJSON != "" {
		err := json.Unmarshal([]byte(integrationsJSON), &opts.Integrations)
		if err != nil {
			exitWithErr(logger, fmt.Errorf("integrations-json is not valid: %v", err))
		}
	}

	if len(opts.Integrations) == 0 {
		exitWithErr(logger, errors.New("missing integrations-json"))
	}

	logWriter, err := pinpointLogWriter(opts.AgentConfig.PinpointRoot)
	if err != nil {
		exitWithErr(logger, err)
	}
	opts.Logger = logger.AddWriter(logWriter)
	return opts.Logger, opts
}

type outputFile struct {
	logger hclog.Logger
	close  io.Closer
	Writer io.Writer
}

func newOutputFile(logger hclog.Logger, cmd *cobra.Command) *outputFile {
	s := &outputFile{}
	s.logger = logger

	v, _ := cmd.Flags().GetString("output-file")
	if v != "" {
		f, err := os.Create(v)
		if err != nil {
			exitWithErr(logger, fmt.Errorf("could not create output-file: %v", err))
		}
		s.close = f
		s.Writer = f
	} else {
		s.Writer = os.Stdout
	}
	return s
}

func (s outputFile) Close() {
	if s.close != nil {
		err := s.close.Close()
		if err != nil {
			exitWithErr(s.logger, fmt.Errorf("could not close the output-file: %v", err))
		}
	}
}

func flagOutputFile(cmd *cobra.Command, name string) {
	cmd.Flags().String("output-file", "", "File to save "+name+" result. Writes to stdout if not specified.")
}
