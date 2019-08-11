package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/pinpt/agent.next/cmd/cmdserviceinstall"
	"github.com/pinpt/agent.next/cmd/cmdserviceuninstall"

	"github.com/pinpt/agent.next/cmd/cmdexport"

	"github.com/pinpt/agent.next/cmd/cmdservicerun"

	"github.com/pinpt/agent.next/pkg/fsconf"

	"github.com/pinpt/agent.next/pkg/deviceinfo"

	"github.com/fatih/color"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/go-ps"
	"github.com/pinpt/agent.next/cmd/cmdenroll"
	"github.com/spf13/cobra"
)

func Execute() {
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

func defaultLogger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Output:     os.Stdout,
		Level:      hclog.Debug,
		JSONFormat: false,
	})
}

func exitWithErr(logger hclog.Logger, err error) {
	logger.Error("error: " + err.Error())
	os.Exit(1)
}

func getPinpointRoot(cmd *cobra.Command) (string, error) {
	res, _ := cmd.Flags().GetString("pinpoint-root")
	if res != "" {
		return res, nil
	}
	return fsconf.DefaultRoot()
}

var cmdEnroll = &cobra.Command{
	Use:   "enroll <code>",
	Short: "Enroll the agent with the Pinpoint Cloud",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		code := args[0]
		logger := defaultLogger()
		pinpointRoot, err := getPinpointRoot(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}
		channel, _ := cmd.Flags().GetString("channel")
		deviceID := deviceinfo.DeviceID()
		ctx := context.Background()
		err = cmdenroll.Run(ctx, cmdenroll.Opts{
			Logger:       logger,
			PinpointRoot: pinpointRoot,
			Code:         code,
			Channel:      channel,
			DeviceID:     deviceID,
		})
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func flagPinpointRoot(cmd *cobra.Command) {
	cmd.Flags().String("pinpoint-root", "", "Custom location of pinpoint work dir.")
}

func init() {
	cmd := cmdEnroll
	flagPinpointRoot(cmd)
	cmd.Flags().String("channel", "dev", "Cloud channel to use.")
	cmdRoot.AddCommand(cmd)
}

var cmdExport = &cobra.Command{
	Use:   "export",
	Short: "Run export passing configured integrations via command line",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()
		opts := cmdexport.Opts{}

		agentConfigJSON, _ := cmd.Flags().GetString("agent-config-json")
		if agentConfigJSON != "" {
			err := json.Unmarshal([]byte(agentConfigJSON), &opts.AgentConfig)
			if err != nil {
				exitWithErr(logger, fmt.Errorf("integrations-json is not valid: %v", err))
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

		integrationsJSON, _ := cmd.Flags().GetString("integrations-json")
		if integrationsJSON == "" {
			exitWithErr(logger, errors.New("missing integrations-json"))
		}

		err := json.Unmarshal([]byte(integrationsJSON), &opts.Integrations)
		if err != nil {
			exitWithErr(logger, fmt.Errorf("integrations-json is not valid: %v", err))
		}

		opts.Logger = logger
		err = cmdexport.Run(opts)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdExport
	flagPinpointRoot(cmd)
	cmd.Flags().String("agent-config-json", "", "Agent config as json")
	cmd.Flags().String("integrations-json", "", "Integrations config as json")
	cmd.Flags().String("integrations-dir", "", "Integrations dir")
	cmdRoot.AddCommand(cmdExport)
}

var cmdServiceInstall = &cobra.Command{
	Use:   "service-install",
	Short: "Install OS service of agent",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()
		err := cmdserviceinstall.Run(logger)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmdRoot.AddCommand(cmdServiceInstall)
}

var cmdServiceUninstall = &cobra.Command{
	Use:   "service-uninstall",
	Short: "Uninstall OS service of agent, but keep data and configuration",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()
		err := cmdserviceuninstall.Run(logger)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmdRoot.AddCommand(cmdServiceUninstall)
}

var cmdServiceRun = &cobra.Command{
	Use:   "service-run",
	Short: "This command is called by OS service to run the service.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()
		pinpointRoot, err := getPinpointRoot(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}
		ctx := context.Background()
		opts := cmdservicerun.Opts{}
		opts.Logger = logger
		opts.PinpointRoot = pinpointRoot
		err = cmdservicerun.Run(ctx, opts)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdServiceRun
	flagPinpointRoot(cmd)
	cmdRoot.AddCommand(cmd)
}
