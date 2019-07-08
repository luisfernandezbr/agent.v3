package fsconf

import "path/filepath"

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

	LastProcessedFile string
}

func j(parts ...string) string {
	return filepath.Join(parts...)
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

	return s
}
