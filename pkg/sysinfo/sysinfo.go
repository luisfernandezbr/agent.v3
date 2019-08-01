package sysinfo

import (
	"os"
	"runtime"

	"github.com/denisbrodbeck/machineid"
)

// SystemInfo returns the operating system details
type SystemInfo struct {
	ID           string
	Name         string `json:"name"`
	Version      string `json:"version"`
	Hostname     string `json:"hostname"`
	Memory       uint64 `json:"memory"`
	NumCPU       int    `json:"num_cpu"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	GoVersion    string `json:"go_version"`
	AgentVersion string `json:"agent_version"`
	FreeSpace    uint64 `json:"free_space"`
}

func getDefault() SystemInfo {
	var s SystemInfo
	hostname, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	id := os.Getenv("PP_AGENT_ID") // for the cloud agent, allow it to be overriden
	if id == "" {
		id, _ = machineid.ProtectedID("pinpoint-agent")
	}
	s.ID = id
	s.Hostname = hostname
	s.Memory = m.Alloc
	s.OS = runtime.GOOS
	s.NumCPU = runtime.NumCPU()
	s.GoVersion = runtime.Version()
	s.Architecture = runtime.GOARCH
	s.AgentVersion = os.Getenv("PP_AGENT_VERSION")
	dir := os.Getenv("PP_CACHEDIR")
	if dir == "" {
		dir, _ = os.Getwd()
	}
	s.FreeSpace = getFreeSpace(dir)
	return s
}
