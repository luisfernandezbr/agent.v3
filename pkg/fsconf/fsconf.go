package fsconf

import (
	"os"
	"path/filepath"

	"github.com/pinpt/go-common/fileutil"

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
	s.LogsIntegrations = j(s.Root, "logs/integrations")
	s.Integrations = j(s.Root, "integrations")
	if !fileutil.FileExists(s.Integrations) {
		s.Integrations = filepath.Join(filepath.Dir(os.Args[0]), "integrations")
	}

	s.RepoCache = j(s.Cache, "repos")
	s.State = j(s.Root, "state")
	s.Uploads = j(s.State, "uploads")
	s.UploadZips = j(s.State, "upload-zips")
	s.RipsrcCheckpoints = j(s.State, "ripsrc_checkpoints")

	s.Config2 = j(s.Root, "config.json")
	s.LastProcessedFile = j(s.State, "last_processed.json")

	return s
}
