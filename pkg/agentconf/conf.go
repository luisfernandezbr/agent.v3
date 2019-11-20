package agentconf

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pinpt/agent.next/pkg/fs"
)

type Config struct {
	APIKey     string `json:"api_key"`
	Channel    string `json:"channel"`
	CustomerID string `json:"customer_id"`
	DeviceID   string `json:"device_id"`
	// SystemID normally does not change across installs on the same machine. But to be safe we keep it in config as well.
	SystemID        string `json:"system_id"`
	PPEncryptionKey string `json:"pp_encryption_key"`
	IntegrationsDir string `json:"integrations-dir"`

	// ExtraIntegrations defines additional integrations that will run on every export trigger in service-run. This is needed to run a custom integration for one of our customers. You need to add these custom integrations to config manually after enroll.
	ExtraIntegrations []struct {
		Name   string                 `json:"name"`
		Config map[string]interface{} `json:"config"`
	} `json:"extra_integrations"`
}

func Save(c Config, loc string) error {
	err := os.MkdirAll(filepath.Dir(loc), 0777)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "")
	if err != nil {
		return err
	}
	return fs.WriteToTempAndRename(bytes.NewReader(b), loc)
}

func Load(loc string) (res Config, _ error) {
	b, err := ioutil.ReadFile(loc)
	if err != nil {
		return res, err
	}
	err = json.Unmarshal(b, &res)
	if err != nil {
		return res, err
	}
	return
}
