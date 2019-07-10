package cmd

import (
	"os"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/go-ps"
	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/agent.next/cmd/cmdserviceinstall"
	"github.com/pinpt/agent.next/cmd/cmdservicerun"
	"github.com/pinpt/agent.next/cmd/cmdserviceuninstall"
	"github.com/pinpt/agent.next/pkg/agentconf"
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

func setupDefaultLoggerFlags(cmd *cobra.Command) {
	cmd.Flags().String("config", "", "Config file to use.")
	cmd.Flags().Bool("config-no-encryption", false, "Use default location for config file, but disable encryption.")
	cmd.Flags().String("config-encryption-key-access", "", "Provide a script to call to get/set encryption key from custom storage.")
}

func defaultConfig(cmd *cobra.Command) (*agentconf.Config, error) {
	opts := agentconf.Opts{}
	opts.File, _ = cmd.Flags().GetString("config")
	opts.NoEncryption, _ = cmd.Flags().GetBool("config-no-encryption")
	opts.EncryptionKeyAccess, _ = cmd.Flags().GetString("config-encryption-key-access")
	return agentconf.New(opts)
}

func exitWithErr(logger hclog.Logger, err error) {
	logger.Error("error: " + err.Error())
	os.Exit(1)
}

var cmdExport = &cobra.Command{
	Use:   "export",
	Short: "Run data export for configured integrations",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()
		config, err := defaultConfig(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}
		err = cmdexport.Run(cmdexport.Opts{
			Logger: logger,
			Config: config,
		})
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	setupDefaultLoggerFlags(cmdExport)
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
		err := cmdservicerun.Run(logger)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmdRoot.AddCommand(cmdServiceRun)
}
