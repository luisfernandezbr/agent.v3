package main

import (
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/service"
	"github.com/spf13/cobra"
)

var serviceNames = service.Names{
	Name:        "serviceexample3",
	DisplayName: "Pinpoint Service Test",
	Description: "Test description",
}

func main() {
	cmdRoot.Execute()
}

var cmdRoot = &cobra.Command{
	Use:              "serviceexample",
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func newLogger() hclog.Logger {
	opts := hclog.DefaultOptions
	//opts.Output = os.Stdout
	return hclog.New(opts)
}

func exitWithErr(logger hclog.Logger, err error) {
	logger.Error("error: " + err.Error())
	os.Exit(1)
}

var cmdInstall = &cobra.Command{
	Use:   "install",
	Short: "Install",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := newLogger()
		err := service.Install(logger, serviceNames, []string{"run"})
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdInstall
	cmdRoot.AddCommand(cmd)
}

var cmdControl = &cobra.Command{
	Use:   "control",
	Short: "control",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger := newLogger()
		err := service.Control(logger, serviceNames, service.ControlAction(args[0]))
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdControl
	cmdRoot.AddCommand(cmd)
}

var cmdRun = &cobra.Command{
	Use:   "run",
	Short: "run",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := newLogger()
		service.Run(serviceNames, service.Opts{}, func(cancel chan bool) error {
			logger.Info("waiting for cancellation")
			<-cancel
			logger.Info("cancelled")
			return nil
		})
	},
}

func init() {
	cmd := cmdRun
	cmdRoot.AddCommand(cmd)
}
