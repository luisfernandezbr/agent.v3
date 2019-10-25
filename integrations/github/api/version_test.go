package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractMajorVersion(t *testing.T) {
	got, err := extractMajorVersion("https://developer.github.com/enterprise/2.18/v4")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "2.18", got)
}
