package sysinfo

import (
	"os"
	"runtime"
	"testing"

	"github.com/denisbrodbeck/machineid"
	"github.com/stretchr/testify/assert"
)

func TestGetDefault(t *testing.T) {
	response := getDefault("")
	answer := myDefaultInfo()
	response.Memory = 0
	response.FreeSpace = 0
	response.TotalMemory = 0
	assert.Equal(t, response, answer)
}

func myDefaultInfo() SystemInfo {
	var s SystemInfo
	hostname, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	id, _ := machineid.ProtectedID("pinpoint-agent")
	s.ID = id
	s.Hostname = hostname
	s.OS = runtime.GOOS
	s.NumCPU = runtime.NumCPU()
	s.GoVersion = runtime.Version()[2:]
	s.Architecture = runtime.GOARCH
	s.AgentVersion = os.Getenv("PP_AGENT_VERSION")
	return s
}
