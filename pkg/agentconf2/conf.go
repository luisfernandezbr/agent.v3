package agentconf2

import (
	"bytes"
	"encoding/json"

	"github.com/pinpt/agent.next/pkg/fs"
)

type Config struct {
	APIKey     string
	Channel    string
	CustomerID string
	DeviceID   string
}

func Save(c Config, loc string) error {
	b, err := json.MarshalIndent(c, "", "")
	if err != nil {
		return err
	}
	return fs.WriteToTempAndRename(bytes.NewReader(b), loc)
}
