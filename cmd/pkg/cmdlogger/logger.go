package cmdlogger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/filelog"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/spf13/cobra"
)

func Stdout(cmd *cobra.Command) hclog.Logger {
	return hclog.New(optsFromCommand(cmd))
}

func optsFromCommand(cmd *cobra.Command) *hclog.LoggerOptions {
	opts := &hclog.LoggerOptions{
		Output: os.Stdout,
	}

	res, _ := cmd.Flags().GetString("log-format")
	if res == "json" {
		opts.JSONFormat = true
	}
	res, _ = cmd.Flags().GetString("log-level")
	switch res {
	case "debug":
		opts.Level = hclog.Debug
	case "info":
		opts.Level = hclog.Info
	default:
		opts.Level = hclog.Debug
	}
	return opts
}

func CopyToFile(cmd *cobra.Command, logger hclog.Logger, pinpointRoot string) (_ hclog.Logger, _ hclog.Level, ok bool) {
	if pinpointRoot == "" {
		var err error
		pinpointRoot, err = fsconf.DefaultRoot()
		if err != nil {
			logger.Error("could not get default pinpoint-root", "err", err)
			return
		}
	}
	fsloc := fsconf.New(pinpointRoot)
	if len(os.Args) <= 1 {
		logger.Error("could not create log file, len(os.Args) <= 1, and we use subcommand as name")
		return
	}
	logFile := filepath.Join(fsloc.Logs, os.Args[1])
	wr, err := filelog.NewSyncWriter(logFile)
	if err != nil {
		logger.Error("could not create log file", "err", err)
		return
	}
	opts := optsFromCommand(cmd)
	opts.Output = io.MultiWriter(os.Stdout, wr)
	res := hclog.New(opts)
	res.Info("initialized logger", "cmd", os.Args[1], "file", logFile)
	return res, opts.Level, true

}
