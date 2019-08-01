package fsconf

import (
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

type Locs struct {
	// Dirs

	Root              string
	Temp              string
	Cache             string
	RepoCache         string
	State             string
	Uploads           string
	RipsrcCheckpoints string
	Logs              string

	// Special files
	Config2           string // new config that is populated from enroll, not for manual editing
	LastProcessedFile string
}

func j(parts ...string) string {
	return filepath.Join(parts...)
}

func DefaultRoot() (string, error) {
	dir, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".pinpoint", "next"), nil
}

func New(pinpointRoot string) Locs {
	if pinpointRoot == "" {
		panic("provide pinpoint root")
	}
	s := Locs{}
	s.Root = pinpointRoot
	s.Temp = j(s.Root, "temp")
	s.Cache = j(s.Root, "cache")
	s.Logs = j(s.Root, "logs")

	s.RepoCache = j(s.Cache, "repos")
	s.State = j(s.Root, "state")
	s.Uploads = j(s.State, "uploads")
	s.RipsrcCheckpoints = j(s.State, "ripsrc_checkpoints")
	s.LastProcessedFile = j(s.State, "last_processed.json")

	s.Config2 = j(s.Root, "config.json")

	return s
}
