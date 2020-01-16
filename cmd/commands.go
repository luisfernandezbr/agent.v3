package cmd

import (
	"context"
	"errors"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdenroll"
	"github.com/pinpt/agent/cmd/cmdexport"
	"github.com/pinpt/agent/cmd/cmdexportonboarddata"
	"github.com/pinpt/agent/cmd/cmdrun"
	"github.com/pinpt/agent/cmd/cmdserviceinstall"
	"github.com/pinpt/agent/cmd/cmdservicerunnorestarts"
	"github.com/pinpt/agent/cmd/cmdvalidate"
	"github.com/pinpt/agent/cmd/cmdvalidateconfig"
	"github.com/pinpt/agent/cmd/pkg/cmdlogger"
	"github.com/pinpt/agent/rpcdef"
	pos "github.com/pinpt/go-common/os"
	"github.com/spf13/cobra"
)

func isInsideDocker() bool {
	return pos.IsInsideContainer()
}

var cmdEnrollNoServiceRun = &cobra.Command{
	Use:   "enroll-no-service-run <code>",
	Short: "Enroll the agent with the Pinpoint Cloud",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger, pinpointRoot := defaultCommandWithFileLogger(cmd)
		code := args[0]

		// once we have pinpoint root, we can also log to a file
		logWriter, err := pinpointLogWriter(pinpointRoot)
		if err != nil {
			exitWithErr(logger, err)
		}
		logger = logger.AddWriter(logWriter)

		runEnroll(cmd, logger, pinpointRoot, code)
	},
}

func runEnroll(cmd *cobra.Command, logger hclog.Logger, pinpointRoot string, code string) {
	channel, _ := cmd.Flags().GetString("channel")
	ctx := context.Background()
	skipValidate, _ := cmd.Flags().GetBool("skip-validate")

	integrationsDir, _ := cmd.Flags().GetString("integrations-dir")
	skipEnroll, _ := cmd.Flags().GetBool("skip-enroll-if-found")

	err := cmdenroll.Run(ctx, cmdenroll.Opts{
		Logger:            logger,
		PinpointRoot:      pinpointRoot,
		IntegrationsDir:   integrationsDir,
		Code:              code,
		Channel:           channel,
		SkipEnrollIfFound: skipEnroll,
		SkipValidate:      skipValidate,
	})
	if err != nil {
		exitWithErr(logger, err)
	}
}

func init() {
	cmd := cmdEnrollNoServiceRun
	flagsLogger(cmd)
	flagPinpointRoot(cmd)
	cmd.Flags().String("integrations-dir", defaultIntegrationsDir(), "Integrations dir")
	cmd.Flags().String("channel", "stable", "Cloud channel to use.")
	cmd.Flags().Bool("skip-validate", false, "skip minimum requirements")
	cmd.Flags().Bool("skip-enroll-if-found", false, "skip enroll if the config is already found")
	cmdRoot.AddCommand(cmd)
}

type runType string

const (
	direct  runType = "direct"
	service runType = "service"
)

var cmdEnroll = &cobra.Command{
	Use:   "enroll <code>",
	Short: "Enroll the agent with the Pinpoint Cloud",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		skipRun, _ := cmd.Flags().GetBool("skip-run")
		if skipRun {
			cmdEnrollNoServiceRun.Run(cmd, args)
			return
		}

		logger, pinpointRoot := defaultCommandWithFileLogger(cmd)
		code := args[0]

		skipValidate, _ := cmd.Flags().GetBool("skip-validate")
		skipEnroll, _ := cmd.Flags().GetBool("skip-enroll-if-found")
		channel, _ := cmd.Flags().GetString("channel")
		integrationsDir, _ := cmd.Flags().GetString("integrations-dir")
		runTypeStr, _ := cmd.Flags().GetString("run-type")

		switch runType(runTypeStr) {
		case direct:
			ctx := context.Background()
			opts := cmdrun.Opts{}
			opts.Logger = logger
			opts.PinpointRoot = pinpointRoot
			opts.IntegrationsDir = integrationsDir
			opts.Enroll.Run = true
			opts.Enroll.Code = code
			opts.Enroll.Channel = channel
			opts.Enroll.SkipValidate = skipValidate
			opts.Enroll.SkipEnrollIfFound = skipEnroll
			err := cmdrun.Run(ctx, opts, nil)
			if err != nil {
				exitWithErr(logger, err)
			}
		case service:
			runEnroll(cmd, logger, pinpointRoot, code)
			err := cmdserviceinstall.Run(logger, pinpointRoot, true)
			if err != nil {
				exitWithErr(logger, err)
			}
		default:
			err := errors.New("run-type should be service or interactive")
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdEnroll
	flagsLogger(cmd)
	flagPinpointRoot(cmd)
	cmd.Flags().String("integrations-dir", defaultIntegrationsDir(), "Integrations dir")
	cmd.Flags().String("channel", "stable", "Cloud channel to use.")
	cmd.Flags().Bool("skip-validate", false, "skip minimum requirements")
	cmd.Flags().Bool("skip-run", false, "Set to true to skip service run. Will need to run it separately.")
	cmd.Flags().Bool("skip-enroll-if-found", false, "skip enroll if the config is already found")
	cmd.Flags().String("run-type", "service", "run the agent either interactive or as a service")
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

		outputFile := newOutputFile(logger, cmd)
		defer outputFile.Close()
		opts.Output = outputFile.Writer

		err := cmdexport.Run(opts)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdExport
	integrationCommandFlags(cmd)
	flagOutputFile(cmd)
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

var cmdServiceRunNoRestarts = &cobra.Command{
	Use:   "service-run-no-restarts",
	Short: "This command is called by OS service to run the service.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger, pinpointRoot := defaultCommandWithFileLogger(cmd)
		ctx := context.Background()
		opts := cmdservicerunnorestarts.Opts{}
		opts.Logger = logger
		opts.LogLevelSubcommands = logger.Level
		opts.PinpointRoot = pinpointRoot
		err := cmdservicerunnorestarts.Run(ctx, opts)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdServiceRunNoRestarts
	flagsLogger(cmd)
	flagPinpointRoot(cmd)
	cmdRoot.AddCommand(cmd)
}

var cmdRun = &cobra.Command{
	Use:   "run",
	Short: "This command is called by OS service to run the service.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// only json is supported as log format for run command, since it proxies the logs from subcommands, from which export is required to be json to be sent to the server corretly
		cmd.Flags().Set("log-format", "json")

		logger := cmdlogger.NewLogger(cmd)
		pinpointRoot, err := getPinpointRoot(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}

		ctx := context.Background()
		opts := cmdrun.Opts{}
		opts.Logger = logger
		opts.PinpointRoot = pinpointRoot
		err = cmdrun.Run(ctx, opts, nil)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdRun
	flagsLogger(cmd)
	flagPinpointRoot(cmd)
	cmdRoot.AddCommand(cmd)
}

var cmdVersion = &cobra.Command{
	Use:   "version",
	Short: "Display the build version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version:", Version)
		fmt.Println("Commit:", Commit)
	},
}

func init() {
	cmdRoot.AddCommand(cmdVersion)
}

var cmdValidate = &cobra.Command{
	Use:   "validate",
	Short: "Validate minimum hardware requirements",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {

		ctx := context.Background()
		logger := cmdlogger.NewLogger(cmd)
		pinpointRoot, err := getPinpointRoot(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}

		if _, err := cmdvalidate.Run(ctx, logger, pinpointRoot); err != nil {
			exitWithErr(logger, err)
		}

	},
}

func init() {
	cmd := cmdValidate
	integrationCommandFlags(cmd)
	cmdRoot.AddCommand(cmd)
}
