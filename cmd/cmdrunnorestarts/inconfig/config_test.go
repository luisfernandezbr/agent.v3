package inconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/agent/pkg/encrypt"
	"github.com/pinpt/integration-sdk/agent"
)

func TestAuthFromEvent(t *testing.T) {
	config := `{
		"api_token": "t1",
		"url": "u1",
		"excluded_repos":["e1"],
		"included_repos":["e1","e2"]
	}`

	encryptionKey, err := encrypt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	data, err := encrypt.EncryptString(config, encryptionKey)

	e := agent.ExportRequestIntegrations{}
	e.Name = "github"
	e.Authorization.Authorization = pstrings.Pointer(data)
	e.Exclusions = []string{"e1"}
	e.Inclusions = []string{"e1", "e2"}

	got, err := AuthFromEvent(e.ToMap(), encryptionKey)
	if err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)
	want := IntegrationAgent{}
	want.Name = "github"
	want.Config.Exclusions = []string{"e1"}
	want.Config.Inclusions = []string{"e1", "e2"}
	want.Config.APIKey = "t1"
	want.Config.URL = "u1"

	assert.Equal(want, got)
}
