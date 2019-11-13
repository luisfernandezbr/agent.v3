package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractMajorVersion(t *testing.T) {
	cases := []struct {
		In   string
		Want string
	}{
		{In: "https://developer.github.com/enterprise/2.18/v4", Want: "2.18"},
		{In: "https://developer.github.com/enterprise/2.18/v3/#authentication", Want: "2.18"},
	}

	for _, c := range cases {
		got, err := extractMajorVersion(c.In)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, c.Want, got)
	}
}
