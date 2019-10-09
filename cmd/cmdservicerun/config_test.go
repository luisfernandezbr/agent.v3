package cmdservicerun

import (
	"testing"

	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/stretchr/testify/assert"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/agent.next/pkg/encrypt"
	"github.com/pinpt/integration-sdk/agent"
)

func TestConfigFromEvent(t *testing.T) {
	config := `{
		"api_token": "t1",
		"url": "u1"
	}`

	encryptionKey, err := encrypt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	data, err := encrypt.EncryptString(config, encryptionKey)

	e := agent.ExportRequestIntegrations{}
	e.Name = "github"
	e.Authorization.Authorization = pstrings.Pointer(data)
	e.Exclusions = []string{"e1", "e2"}

	got, err := configFromEvent(e.ToMap(), IntegrationTypeSourcecode, encryptionKey)
	if err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)
	want := cmdintegration.Integration{
		Name: "github",
		Config: map[string]interface{}{
			"apitoken":       "t1",
			"url":            "u1",
			"excluded_repos": []interface{}{"e1", "e2"},
		},
	}

	assert.Equal(want, got)
}
