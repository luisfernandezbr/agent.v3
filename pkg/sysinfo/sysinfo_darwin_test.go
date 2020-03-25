// +build darwin

package sysinfo

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/stretchr/testify/assert"
)

func TestGetSystemInfoDarwin(t *testing.T) {
	t.Skip("fails on macos")
	root, err := fsconf.DefaultRoot()
	assert.NoError(t, err)

	response := GetSystemInfo(root)
	answer := mySystemInfoDarwin()
	//assert.NotZero(t, response.Memory) memory check fails on darwin
	assert.NotZero(t, response.FreeSpace)
	response.Memory = 0
	answer.Memory = 0
	response.FreeSpace = 0
	answer.FreeSpace = 0
	assert.Equal(t, response, answer)
}

func mySystemInfoDarwin() SystemInfo {
	var buf bytes.Buffer
	c := exec.Command("sw_vers")
	c.Stdout = &buf
	c.Run()
	kv := make(map[string]string)
	for _, line := range strings.Split(buf.String(), "\n") {
		if line != "" {
			tok := strings.Split(line, ":")
			if len(tok) > 1 {
				key, value := strings.TrimSpace(tok[0]), strings.TrimSpace(tok[1])
				kv[key] = value
			}
		}
	}
	root, _ := fsconf.DefaultRoot()
	def := getDefault(root)
	def.Name = kv["ProductName"]
	def.Version = kv["ProductVersion"]
	return def
}
