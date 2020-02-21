package inconfig

import (
	"fmt"

	"github.com/pinpt/integration-sdk/agent"
)

// IntegrationType is the enumeration type for backend system_type

type IntegrationBase struct {
	// ID is the integration id passed from backend. We add it to last processed key to avoid conflicts and to maintain state in case ordering of integrations changes in export.
	// Can be left empty when export is called manually in that case will use index in integration array instead.
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Type IntegrationType `json:"type"` // sourcecode or work
}

func (s IntegrationBase) IntegrationDef() (res IntegrationDef) {
	res.Name = s.Name
	res.Type = s.Type
	return
}

type Integration struct {
	IntegrationBase
	Config map[string]interface{} `json:"config"`
}

type IntegrationAgent struct {
	IntegrationBase
	Config IntegrationConfigAgent `json:"config"`
}

type IntegrationType agent.IntegrationRequestIntegrationSystemType

const IntegrationTypeWork = IntegrationType(agent.IntegrationRequestIntegrationSystemTypeWork)
const IntegrationTypeSourcecode = IntegrationType(agent.IntegrationRequestIntegrationSystemTypeSourcecode)
const IntegrationTypeCodequality = IntegrationType(agent.IntegrationRequestIntegrationSystemTypeCodequality)
const IntegrationTypeUser = IntegrationType(agent.IntegrationRequestIntegrationSystemTypeUser)

func (in IntegrationType) String() string {
	return agent.IntegrationRequestIntegrationSystemType(in).String()
}

type IntegrationConfigAgent struct {
	// URL URL of instance if relevant
	URL string `json:"url"`
	// Username Username for instance, if relevant
	Username string `json:"username"`
	// Password Password for instance, if relevant
	Password string `json:"password"`
	// CollectionName Collection name for instance, if relevant
	CollectionName string `json:"collection_name"`
	// APIKey API Key for instance, if relevant
	APIKey string `json:"api_key"`
	// Hostname Hostname for instance, if relevant
	Hostname string `json:"hostname"`
	// APIVersion the api version of the integration
	APIVersion string `json:"api_version"`
	// Organization Organization for instance, if relevant
	Organization string `json:"organization"`
	// Exclusions list of exclusions
	Exclusions []string `json:"exclusions"`
	// Exclusions list of inclusions
	Inclusions []string `json:"inclusions"`
	// AccessToken Access token
	AccessToken string `json:"access_token"`
	// RefreshToken Refresh token
	RefreshToken string `json:"refresh_token"`
}

// IntegrationDef defines a unique integration.
// Since some integration binaries contain different
// integrations based on type, we neeed to include type as well.
type IntegrationDef struct {
	// Name is the name of the integration binary
	Name string

	// Type is the value of the type option passed to binary.
	// Can be empty if binary contains only one integration.
	Type IntegrationType
}

func (s IntegrationDef) Empty() bool {
	return s.String() == ""
}

func (s IntegrationDef) String() string {
	if s.Type == -1 || s.Type.String() == "" || s.Type.String() == "unset" {
		return s.Name
	}
	return s.Name + "@" + s.Type.String()
}

func TypeFromString(t string) (IntegrationType, error) {
	switch t {
	case "WORK":
		return IntegrationType(agent.IntegrationRequestIntegrationSystemTypeWork), nil
	case "SOURCECODE":
		return IntegrationType(agent.IntegrationRequestIntegrationSystemTypeSourcecode), nil
	case "CODEQUALITY":
		return IntegrationType(agent.IntegrationRequestIntegrationSystemTypeCodequality), nil
	case "USER":
		return IntegrationType(agent.IntegrationRequestIntegrationSystemTypeUser), nil
	}
	return IntegrationType(-1), fmt.Errorf("invalid integration id type: %v", t)
}
