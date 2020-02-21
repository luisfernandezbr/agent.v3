// Package expin defined export integration ID that is used to distinguish between different integrations queued for export
package expin

import (
	"strconv"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
)

// Index is integration index in export request
type Index int

// Export contains index and info on the integration that is running. Useful for pass around for logging and debugging.
type Export struct {
	Index       Index
	Integration inconfig.IntegrationDef
}

func (s Export) String() string {
	return s.Integration.String() + "@" + strconv.Itoa(int(s.Index))
}

func (s Export) Empty() bool {
	return s.Integration.Empty()
}
