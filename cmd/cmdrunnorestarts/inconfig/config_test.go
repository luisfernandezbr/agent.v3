package inconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pstrings "github.com/pinpt/go-common/v10/strings"

	"github.com/pinpt/agent/pkg/encrypt"
	"github.com/pinpt/integration-sdk/agent"
)

func TestAuthFromEvent(t *testing.T) {
	config := `{
		"api_token": "t1",
		"url": "github.com"
	}`

	encryptionKey, err := encrypt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	data, err := encrypt.EncryptString(config, encryptionKey)

	e := agent.ExportRequestIntegrations{}
	e.ID = "id1"
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
	want.ID = "id1"
	want.Name = "github"
	want.Config.Exclusions = []string{"e1"}
	want.Config.Inclusions = []string{"e1", "e2"}
	want.Config.APIKey = "t1"
	want.Config.URL = "https://github.com"

	assert.Equal(want, got)
}

func TestURLAddHTTPSPrefix(t *testing.T) {
	cases := []struct {
		URL  string
		Want string
	}{
		{"", ""},
		{"github.com", "https://github.com"},
		{"http://github.com", "http://github.com"},
		{"https://github.com", "https://github.com"},
	}
	for _, c := range cases {
		got := addHTTPSPrefix(c.URL)
		if got != c.Want {
			t.Errorf("invalid result for case %v want %v got %v", c.URL, c.Want, got)
		}
	}
}
