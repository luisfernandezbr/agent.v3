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
	"github.com/pinpt/agent/cmd/cmdrunnorestarts"
	"github.com/pinpt/agent/cmd/cmdserviceinstall"
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

func runEnroll(cmd *cobra.Command, logger hclog.Logger, pinpointRoot string, code string) {
	channel, _ := cmd.Flags().GetString("channel")
	skipValidate, _ := cmd.Flags().GetBool("skip-validate")
	integrationsDir, _ := cmd.Flags().GetString("integrations-dir")
	skipEnroll, _ := cmd.Flags().GetBool("skip-enroll-if-found")

	err := cmdenroll.Run(context.Background(), cmdenroll.Opts{
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

type runType string

const (
	rtDirect     runType = "direct"
	rtService            = "service"
	rtEnrollOnly         = "enroll-only"
)

var cmdEnroll = &cobra.Command{
	Use:   "enroll <code>",
	Short: "Enroll the agent with the Pinpoint Cloud",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger, pinpointRoot := defaultCommandWithFileLogger(cmd)
		code := args[0]

		runTypeStr, _ := cmd.Flags().GetString("run-type")

		channel, _ := cmd.Flags().GetString("channel")
		skipValidate, _ := cmd.Flags().GetBool("skip-validate")
		integrationsDir, _ := cmd.Flags().GetString("integrations-dir")

		enrollOpts := cmdenroll.Opts{
			Logger:            logger,
			PinpointRoot:      pinpointRoot,
			IntegrationsDir:   integrationsDir,
			Code:              code,
			Channel:           channel,
			SkipEnrollIfFound: false,
			SkipValidate:      skipValidate,
		}

		runEnroll := func() {
			err := cmdenroll.Run(context.Background(), enrollOpts)
			if err != nil {
				exitWithErr(logger, err)
			}
		}

		switch runType(runTypeStr) {
		case rtEnrollOnly:
			runEnroll()
			return
		case rtDirect:
			enrollOpts.SkipEnrollIfFound = true
			runEnroll()

			ctx := context.Background()
			opts := cmdrun.Opts{}
			opts.Logger = logger
			opts.PinpointRoot = pinpointRoot
			opts.IntegrationsDir = integrationsDir
			err := cmdrun.Run(ctx, opts, nil)
			if err != nil {
				exitWithErr(logger, err)
			}
		case rtService:
			runEnroll()
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

	cmd.Flags().String("channel", "stable", "Cloud channel to use.")
	cmd.Flags().Bool("skip-validate", false, "Skip hardware/software requirements check.")

	cmd.Flags().String("integrations-dir", defaultIntegrationsDir(), "Custom directory for integrations binaries.")
	cmd.Flags().String("run-type", "direct", `One of service, direct, enroll-only. "service" installs agent as OS service. "direct" runs the agent directly after enrolling using one command, which is useful for docker. "enroll-only" does not run the service, use separate run command in this case.`)

	cmd.Flags().Bool("skip-enroll-if-found", false, "Deprecated.")

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

		outputFile, _ := cmd.Flags().GetString("output-file")
		if outputFile != "" {
			outputFile := newOutputFile(logger, cmd)
			defer outputFile.Close()
			opts.Output = outputFile.Writer
		}

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

func runNoRestarts(cmd *cobra.Command, args []string) {
	logger, pinpointRoot := defaultCommandWithFileLogger(cmd)
	ctx := context.Background()
	opts := cmdrunnorestarts.Opts{}
	opts.Logger = logger
	opts.LogLevelSubcommands = logger.Level
	opts.PinpointRoot = pinpointRoot
	err := cmdrunnorestarts.Run(ctx, opts)
	if err != nil {
		exitWithErr(logger, err)
	}
}

func runWithRestarts(cmd *cobra.Command, args []string) {
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
}

var cmdRun = &cobra.Command{
	Use:   "run",
	Short: "Run the agent directly without using os service",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		noRestarts, _ := cmd.Flags().GetBool("no-restarts")
		if noRestarts {
			runNoRestarts(cmd, args)
			return
		}
		runWithRestarts(cmd, args)
	},
}

func init() {
	cmd := cmdRun
	flagsLogger(cmd)
	flagPinpointRoot(cmd)
	cmd.Flags().Bool("no-restarts", false, "By default run restarts on errors and panics, set to true to avoid restarting.")
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
