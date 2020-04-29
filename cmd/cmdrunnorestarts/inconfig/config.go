package inconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/agent/pkg/encrypt"
	"github.com/pinpt/agent/pkg/structmarshal"
)

// AuthFromEvent converts the config received from backend export or onboarding events and
// converts it to config that integrations can accept (inconfig.IntegrationAgent).
// Also requires encryptionKey to decrypt the auth data.
func AuthFromEvent(data map[string]interface{}, encryptionKey string) (in IntegrationAgent, err error) {
	var obj struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Authorization struct {
			Authorization string `json:"authorization"`
		} `json:"authorization"`
		Exclusions []string `json:"exclusions"`
		Inclusions []string `json:"inclusions"`
	}

	err = structmarshal.MapToStruct(data, &obj)
	if err != nil {
		return
	}

	authEncr := obj.Authorization.Authorization
	if authEncr == "" {
		err = errors.New("missing encrypted auth data")
		return
	}

	auth, err := encrypt.DecryptString(authEncr, encryptionKey)
	if err != nil {
		err = fmt.Errorf("could not decrypt Authorization field in event for %v integration: %v", obj.Name, err)
		return
	}

	err = json.Unmarshal([]byte(auth), &in.Config)
	if err != nil {
		return
	}
	// TODO: Rename "api_token" in the Admin UI and remove from the agent.IntegrationRequestIntegration object
	var workaround struct {
		APIToken1 string `json:"api_token"`
		APIToken2 string `json:"apitoken"`
	}
	err = json.Unmarshal([]byte(auth), &workaround)
	if err != nil {
		return
	}
	if workaround.APIToken1 != "" {
		in.Config.APIKey = workaround.APIToken1
	}
	if workaround.APIToken2 != "" {
		in.Config.APIKey = workaround.APIToken2
	}

	in.ID = obj.ID
	in.Name = obj.Name
	in.Config.Inclusions = obj.Inclusions
	in.Config.Exclusions = obj.Exclusions
	err = ConvertEdgeCases(&in)

	if in.ID == "" {
		err = errors.New("missing integration id")
		return
	}
	if in.Name == "" {
		err = errors.New("missing integration name")
		return
	}
	return
}

// TODO: the backend should send us the correct data for each integration
func ConvertEdgeCases(in *IntegrationAgent) error {

	if in.Name == "jira" {
		if in.Config.URL == "" {
			return errors.New("missing jira url in config")
		}
		u, err := url.Parse(in.Config.URL)
		if err != nil {
			return fmt.Errorf("invalid jira url: %v", err)
		}
		in.Name = "jira-hosted"
		if strings.HasSuffix(u.Host, ".atlassian.net") || strings.HasSuffix(u.Host, ".jira.com") {
			in.Name = "jira-cloud"
		}
	}
	return nil
}
