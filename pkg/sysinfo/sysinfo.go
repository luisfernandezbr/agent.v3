package sysinfo

import (
	"os"
	"runtime"

	sigar "github.com/cloudfoundry/gosigar"
	"github.com/denisbrodbeck/machineid"
	pos "github.com/pinpt/go-common/os"
)

var root string

func SetRoot(r string) {
	root = r
}

// SystemInfo returns the operating system details
type SystemInfo struct {
	ID           string
	Name         string `json:"name"`
	Version      string `json:"version"`
	Hostname     string `json:"hostname"`
	Memory       uint64 `json:"memory"`
	TotalMemory  uint64 `json:"total_memory"`
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
	id := pos.Getenv("PP_AGENT_ID", os.Getenv("PP_UUID")) // for the cloud agent, allow it to be overriden
	if id == "" {
		id, _ = machineid.ProtectedID("pinpoint-agent")
	}
	mem := sigar.Mem{}
	mem.Get()
	s.ID = id
	s.Hostname = hostname
	s.Memory = m.Alloc
	s.TotalMemory = mem.Total
	s.OS = runtime.GOOS
	s.NumCPU = runtime.NumCPU()
	s.GoVersion = runtime.Version()[2:] // trim off go
	s.Architecture = runtime.GOARCH
	s.AgentVersion = os.Getenv("PP_AGENT_VERSION")
	s.FreeSpace = getFreeSpace(root)

	return s
}
