// Package expin defined export integration ID that is used to distinguish between different integrations queued for export
package expin

import (
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
)

// Export contains index and info on the integration that is running. Useful for pass around for logging and debugging.
type Export struct {
	// Index is the index of integration in the passed export array
	Index          int
	IntegrationID  string
	IntegrationDef inconfig.IntegrationDef
}

func NewExport(index int, id string, def inconfig.IntegrationDef) Export {
	return Export{
		Index:          index,
		IntegrationID:  id,
		IntegrationDef: def,
	}
}

func (s Export) String() string {
	return s.IntegrationDef.String() + "@" + s.IntegrationID
}

func (s Export) Empty() bool {
	return s.IntegrationDef.Empty()
}
