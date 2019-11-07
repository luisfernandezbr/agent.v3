package fsconf

import (
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
)

type Locs struct {
	// Dirs

	Root             string
	Temp             string
	Cache            string
	Logs             string
	LogsIntegrations string
	Integrations     string

	RepoCache         string
	State             string
	Uploads           string
	UploadZips        string
	RipsrcCheckpoints string

	// Special files
	Config2 string // new config that is populated from enroll, not for manual editing

	// LastProcessedFile stores timestamps or other data to mark last processed objects
	LastProcessedFile string

	// DedupFile contains hashes of all objects sent in incrementals to avoid sending the same objects multiple times
	DedupFile string
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
	s.LogsIntegrations = j(s.Root, "logs/integrations")
	s.Integrations = j(s.Root, "integrations")

	s.RepoCache = j(s.Cache, "repos")
	s.State = j(s.Root, "state")
	s.Uploads = j(s.State, "uploads")
	s.UploadZips = j(s.State, "upload-zips")
	s.RipsrcCheckpoints = j(s.State, "ripsrc_checkpoints")

	s.Config2 = j(s.Root, "config.json")
	s.LastProcessedFile = j(s.State, "last_processed.json")
	s.DedupFile = j(s.State, "dedup.json")

	return s
}
