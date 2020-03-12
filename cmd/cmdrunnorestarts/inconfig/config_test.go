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
		"url": "u1"
	}`

	encryptionKey, err := encrypt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	data, err := encrypt.EncryptString(config, encryptionKey)

	e := agent.ExportRequestIntegrations{}
	e.ID = "id1"
	e.Name = "github"
	e.CreatedByUserID = pstrings.Pointer("user1")
	e.Authorization.Authorization = pstrings.Pointer(data)
	e.Exclusions = []string{"e1"}
	e.Inclusions = []string{"e1", "e2"}

	got, err := AuthFromEvent(e.ToMap(), encryptionKey)
	if err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)
	want := IntegrationAgent{}
	want.ID = "id1"
	want.Name = "github"
	want.CreatedByUserID = "user1"
	want.Config.Exclusions = []string{"e1"}
	want.Config.Inclusions = []string{"e1", "e2"}
	want.Config.APIKey = "t1"
	want.Config.URL = "u1"

	assert.Equal(want, got)
}
