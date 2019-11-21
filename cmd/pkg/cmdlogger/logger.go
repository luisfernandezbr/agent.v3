package cmdlogger

import (
	"io"
	"os"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
)

// Logger the logger object
type Logger struct {
	hclog.Logger
	Level   hclog.Level
	opts    *hclog.LoggerOptions
	writers []io.Writer
	cmdName string
}

// NewLogger Creates a new Logger with default values
func NewLogger(cmd *cobra.Command) Logger {
	s := Logger{
		opts: optsFromCommand(cmd),
	}
	s.cmdName = strings.Split(cmd.Use, " ")[0]

	s.writers = []io.Writer{os.Stdout}
	s.Logger = hclog.New(s.opts).With("comp", s.cmdName)
	s.Level = s.opts.Level
	return s
}

// AddWriter Appends a writer to the Logger
func (s Logger) AddWriter(writer io.Writer) Logger {
	res := s
	// copy to avoid overwriting writers in same underlying array
	wrs := make([]io.Writer, len(s.writers))
	copy(wrs, s.writers)
	wrs = append(wrs, writer)
	res.writers = wrs
	res.opts.Output = io.MultiWriter(res.writers...)
	res.Logger = hclog.New(s.opts).With("comp", s.cmdName)
	return res
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
