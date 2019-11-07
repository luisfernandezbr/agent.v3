// +build linux

package sysinfo

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSystemInfoLinux(t *testing.T) {
	t.Skip("this test fails in docker")

	response := GetSystemInfo("")
	answer := mySystemInfoLinux()
	assert.NotZero(t, response.Memory)
	assert.NotZero(t, response.FreeSpace)
	response.Memory = 0
	answer.Memory = 0
	response.FreeSpace = 0
	answer.FreeSpace = 0
	assert.Equal(t, response, answer)
}

func mySystemInfoLinux() SystemInfo {
	var buf bytes.Buffer
	c := exec.Command("cat /etc/*-release")
	c.Stdout = &buf
	c.Run()
	kv := make(map[string]string)
	for _, line := range strings.Split(buf.String(), "\n") {
		if line != "" {
			tok := strings.Split(line, "=")
			if len(tok) > 1 {
				key, value := strings.TrimSpace(tok[0]), strings.TrimSpace(tok[1])
				kv[key] = value
			}
		}
	}
	def := getDefault("")
	def.Name = kv["NAME"]
	def.Version = kv["VERSION_ID"]
	return def
}
