package main

import (
	"context"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
)

var cmdRoot = &cobra.Command{
	Use:              "agent-dev-mock-for-backend",
	Long:             "agent-dev-mock-for-backend",
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

var cmdEnroll = &cobra.Command{
	Use:   "enroll <code>",
	Short: "Enroll for backend",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()

		code := args[0]
		ctx := context.Background()

		err := enrollRequest(ctx, logger, code, agentOpts{
			DeviceID: "device1",
			Channel:  "dev",
		})

		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmdRoot.AddCommand(cmdEnroll)
}

var cmdRun = &cobra.Command{
	Use:   "run <api_key> <customer_id>",
	Short: "Run",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()

		apiKey := args[0]
		customerID := args[1]
		ctx := context.Background()

		err := runService(ctx, logger, apiKey, customerID, agentOpts{
			DeviceID: "device1",
			Channel:  "dev",
		})

		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmdRoot.AddCommand(cmdRun)
}

func main() {
	cmdRoot.Execute()
}
