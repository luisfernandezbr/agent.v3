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
	s.Logger = hclog.New(s.opts)
	s.Level = s.opts.Level
	return s
}

// AddWriter Appends a writer to the Logger
func (s Logger) AddWriter(writer io.Writer) Logger {
	logger := s
	logger.writers = append(logger.writers, writer)
	logger.opts.Output = io.MultiWriter(logger.writers...)
	logger.Logger = hclog.New(s.opts)
	return logger
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

// Named Create a logger that will prepend the name string on the front of all messages.
func (s Logger) Named(name string) hclog.Logger {
	logger := s
	logger.Logger = logger.Logger.Named(name)
	return logger
}

// With Creates a sublogger that will always have the given key/value pairs
func (s Logger) With(args ...interface{}) hclog.Logger {
	logger := s
	logger.Logger = logger.Logger.With(args...)
	return logger
}

// ResetNamed Create a logger that will prepend the name string on the front of all messages
func (s Logger) ResetNamed(name string) hclog.Logger {
	logger := s
	logger.Logger = logger.Logger.ResetNamed(name)
	return logger
}

// Trace emits a message and key/value pairs at the TRACE level
func (s Logger) Trace(msg string, args ...interface{}) {
	arr := append([]interface{}{"_cmd", s.cmdName}, args...)
	s.Logger.Trace(msg, arr...)
}

// Debug emits a message and key/value pairs at the DEBUG level
func (s Logger) Debug(msg string, args ...interface{}) {
	arr := append([]interface{}{"_cmd", s.cmdName}, args...)
	s.Logger.Debug(msg, arr...)
}

// Info emits a message and key/value pairs at the INFO level
func (s Logger) Info(msg string, args ...interface{}) {
	arr := append([]interface{}{"_cmd", s.cmdName}, args...)
	s.Logger.Info(msg, arr...)
}

// Warn emits a message and key/value pairs at the WARN level
func (s Logger) Warn(msg string, args ...interface{}) {
	arr := append([]interface{}{"_cmd", s.cmdName}, args...)
	s.Logger.Warn(msg, arr...)
}

// Error emits a message and key/value pairs at the ERROR level
func (s Logger) Error(msg string, args ...interface{}) {
	arr := append([]interface{}{"_cmd", s.cmdName}, args...)
	s.Logger.Error(msg, arr...)
}
