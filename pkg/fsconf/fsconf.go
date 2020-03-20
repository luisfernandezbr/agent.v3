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

	RepoCache         string
	State             string
	Uploads           string
	UploadZips        string
	RipsrcCheckpoints string

	Backup                  string
	RipsrcCheckpointsBackup string

	ServiceRunCrashes string

	IntegrationsDefaultDir string

	// Special files
	Config2 string // new config that is populated from enroll, not for manual editing

	// LastProcessedFile stores timestamps or other data to mark last processed objects
	LastProcessedFile       string
	LastProcessedFileBackup string

	// ExportQueueFile stores exports requests
	ExportQueueFile string

	// DedupFile contains hashes of all objects sent in incrementals to avoid sending the same objects multiple times
	DedupFile string

	// CleanupDirs are directories that will be removed on every run
	CleanupDirs []string
}

func j(parts ...string) string {
	return filepath.Join(parts...)
}

func DefaultRoot() (path string, err error) {
	dir, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	path = filepath.Join(dir, ".pinpoint", "next")
	return
}

func New(pinpointRoot string) Locs {
	if pinpointRoot == "" {
		panic("provide pinpoint root")
	}
	s := Locs{}
	s.Root = pinpointRoot
	s.Temp = j(s.Root, "temp")
	s.CleanupDirs = append(s.CleanupDirs, s.Temp)

	s.Cache = j(s.Root, "cache")
	s.Logs = j(s.Root, "logs")
	s.LogsIntegrations = j(s.Root, "logs/integrations")

	s.RepoCache = j(s.Cache, "repos")

	s.CleanupDirs = append(s.CleanupDirs, j(s.Root, "state", "v1"))
	s.State = j(s.Root, "state", "v2")

	s.Uploads = j(s.State, "uploads")
	s.UploadZips = j(s.State, "upload-zips")
	s.Backup = j(s.State, "backup")

	s.RipsrcCheckpoints = j(s.State, "ripsrc_checkpoints/v2")
	s.RipsrcCheckpointsBackup = j(s.Backup, "ripsrc_checkpoints/v2")

	s.ServiceRunCrashes = j(s.Logs, "service-run-crashes")

	s.IntegrationsDefaultDir = j(s.Root, "integrations")

	s.Config2 = j(s.Root, "config.json")
	s.LastProcessedFile = j(s.State, "last_processed.json")
	s.LastProcessedFileBackup = j(s.Backup, "last_processed.json")
	s.ExportQueueFile = j(s.State, "export_queue.json")
	s.DedupFile = j(s.State, "dedup_v2.json")
	return s
}
