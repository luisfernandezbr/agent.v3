package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/fileutil"

	"github.com/pinpt/agent.next/cmd/cmdenroll"
	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/agent.next/cmd/cmdexportonboarddata"
	"github.com/pinpt/agent.next/cmd/cmdserviceinstall"
	"github.com/pinpt/agent.next/cmd/cmdservicerun"
	"github.com/pinpt/agent.next/cmd/cmdserviceuninstall"
	"github.com/pinpt/agent.next/cmd/cmdvalidate"
	"github.com/pinpt/agent.next/cmd/cmdvalidateconfig"
	"github.com/pinpt/agent.next/cmd/pkg/cmdlogger"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/spf13/cobra"
)

func isInsideDocker() bool {
	if fileutil.FileExists("/proc/self/cgroup") {
		buf, _ := ioutil.ReadFile("/proc/self/cgroup")
		if bytes.Contains(buf, []byte("docker")) {
			return true
		}
	}
	return false
}

var cmdEnroll = &cobra.Command{
	Use:   "enroll <code>",
	Short: "Enroll the agent with the Pinpoint Cloud",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		code := args[0]
		logger := cmdlogger.Stdout(cmd)
		pinpointRoot, err := getPinpointRoot(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}

		// once we have pinpoint root, we can also log to a file
		logger, _, ok := cmdlogger.CopyToFile(cmd, logger, pinpointRoot)
		if !ok {
			return
		}

		channel, _ := cmd.Flags().GetString("channel")

		ctx := context.Background()
		err = cmdenroll.Run(ctx, cmdenroll.Opts{
			Logger:       logger,
			PinpointRoot: pinpointRoot,
			Code:         code,
			Channel:      channel,
		})
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdEnroll
	flagsLogger(cmd)
	flagPinpointRoot(cmd)
	cmd.Flags().String("channel", "edge", "Cloud channel to use.")
	cmdRoot.AddCommand(cmd)
}

var cmdExport = &cobra.Command{
	Use:    "export",
	Hidden: true,
	Short:  "Export all data of multiple passed integrations",
	Args:   cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		opts := cmdexport.Opts{}
		logger, opts2 := integrationCommandOpts(cmd)
		opts.Opts = opts2
		opts.ReprocessHistorical, _ = cmd.Flags().GetBool("reprocess-historical")
		err := cmdexport.Run(opts)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdExport
	integrationCommandFlags(cmd)
	cmd.Flags().Bool("reprocess-historical", false, "Set to true to discard incremental checkpoint and reprocess historical instead.")
	cmdRoot.AddCommand(cmd)
}

var cmdValidateConfig = &cobra.Command{
	Use:    "validate-config",
	Hidden: true,
	Short:  "Validates the configuration by making a test connection",
	Args:   cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger, baseOpts := integrationCommandOpts(cmd)
		opts := cmdvalidateconfig.Opts{}
		opts.Opts = baseOpts

		outputFile := newOutputFile(logger, cmd)
		defer outputFile.Close()
		opts.Output = outputFile.Writer

		err := cmdvalidateconfig.Run(opts)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdValidateConfig
	integrationCommandFlags(cmd)
	flagOutputFile(cmd)
	cmdRoot.AddCommand(cmd)
}

var cmdExportOnboardData = &cobra.Command{
	Use:    "export-onboard-data",
	Hidden: true,
	Short:  "Exports users, repos or projects based on param for a specified integration. Saves that data into provided file.",
	Args:   cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger, baseOpts := integrationCommandOpts(cmd)
		opts := cmdexportonboarddata.Opts{}
		opts.Opts = baseOpts

		outputFile := newOutputFile(logger, cmd)
		defer outputFile.Close()
		opts.Output = outputFile.Writer

		{
			v, _ := cmd.Flags().GetString("object-type")
			if v == "" {
				exitWithErr(logger, errors.New("provide object-type arg"))
			}
			if v == "users" || v == "repos" || v == "projects" || v == "workconfig" {
				opts.ExportType = rpcdef.OnboardExportType(v)
			} else {
				exitWithErr(logger, fmt.Errorf("object-type must be one of: users, repos, projects, got %v", v))
			}
		}

		err := cmdexportonboarddata.Run(opts)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdExportOnboardData
	integrationCommandFlags(cmd)
	flagOutputFile(cmd)
	cmd.Flags().String("object-type", "", "Object type to export, one of: users, repos, projects.")
	cmdRoot.AddCommand(cmd)
}

var cmdServiceInstall = &cobra.Command{
	Use:   "service-install",
	Short: "Install OS service of agent",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := cmdlogger.Stdout(cmd)
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
		logger := cmdlogger.Stdout(cmd)
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
		// only json is supported as log format for service-run, since it proxies the logs from subcommands, from which export is required to be json to be sent to the server corretly
		cmd.Flags().Set("log-format", "json")

		logger := cmdlogger.Stdout(cmd)
		pinpointRoot, err := getPinpointRoot(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}
		logger, level, ok := cmdlogger.CopyToFile(cmd, logger, pinpointRoot)
		if !ok {
			return
		}
		ctx := context.Background()
		opts := cmdservicerun.Opts{}
		opts.Logger = logger
		opts.LogLevelSubcommands = level
		opts.PinpointRoot = pinpointRoot
		err = cmdservicerun.Run(ctx, opts)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdServiceRun
	flagsLogger(cmd)
	flagPinpointRoot(cmd)
	cmdRoot.AddCommand(cmd)
}

var cmdVersion = &cobra.Command{
	Use:   "version",
	Short: "Display the build version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}

func init() {
	cmdRoot.AddCommand(cmdVersion)
}

var cmdValidate = &cobra.Command{
	Use:   "validate",
	Short: "Validate mininum hardware requirements",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {

		ctx := context.Background()
		logger := hclog.New(&hclog.LoggerOptions{
			Output:     os.Stdout,
			Level:      hclog.Debug,
			JSONFormat: false,
		})

		if _, err := cmdvalidate.Run(ctx, logger, true); err != nil {
			exitWithErr(logger, err)
		}

	},
}

func init() {
	cmd := cmdValidate
	integrationCommandFlags(cmd)
	cmdRoot.AddCommand(cmd)
}
