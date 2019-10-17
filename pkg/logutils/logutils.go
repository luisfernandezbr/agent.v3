package logutils

import (
	"github.com/hashicorp/go-hclog"
)

func LogLevelToString(lvl hclog.Level) string {
	switch lvl {
	case hclog.Trace:
		return "trace"
	case hclog.Debug:
		return "debug"
	case hclog.Info:
		return "info"
	case hclog.Warn:
		return "warn"
	case hclog.Error:
		return "error"
	default:
		return ""
	}
}
