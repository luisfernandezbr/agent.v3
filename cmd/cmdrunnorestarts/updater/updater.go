// Package updater handles agent updates. It downloads binaries based
// on provided version for both agent and integrations and replaces
// them in place.
// It also downloads built-in integrations if only agent binary is present.
package updater

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	pstrings "github.com/pinpt/go-common/v10/strings"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/build"
	"github.com/pinpt/agent/pkg/fs"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/go-common/v10/api"
)

// Updater handles agent and built-in integration updates
type Updater struct {
	logger  hclog.Logger
	fsconf  fsconf.Locs
	channel string

	integrationsParentDir string
	integrationsSubDir    string
}

// New creates updater
func New(logger hclog.Logger, fslocs fsconf.Locs, conf agentconf.Config) *Updater {
	s := &Updater{}
	s.logger = logger
	s.fsconf = fslocs
	s.channel = conf.Channel
	s.integrationsParentDir = conf.IntegrationsDir
	if s.integrationsParentDir == "" {
		s.integrationsParentDir = fslocs.IntegrationsDefaultDir
	}
	// store downloaded integrations in bin subfolder, to allow updates using folder rename
	// fixes error in docker when using /bin/integrations as integrationsDir
	// Could not update: updateIntegrations: failed to replace integrations: could not rename curr to backup, err: rename /bin/integrations /bin/integrations.old0: invalid cross-device link
	s.integrationsSubDir = filepath.Join(s.integrationsParentDir, "bin")
	return s
}

// DownloadIntegrationsIfMissing downloads integrations if those
// are not present in integrations dir. This would happen
// if use only downloaded the agent binary.
func (s *Updater) DownloadIntegrationsIfMissing() error {
	exists, err := fs.Exists(s.integrationsParentDir)
	if err != nil {
		return fmt.Errorf("Could not read integration dir: %v", err)
	}
	if exists {
		return nil
	}

	version := os.Getenv("PP_AGENT_VERSION")

	s.logger.Info("Integrations dir does not exist, downloading integrations", "dir", s.integrationsParentDir, "version", version)

	err = os.MkdirAll(s.fsconf.Temp, 0777)
	if err != nil {
		return err
	}

	downloadDir, err := ioutil.TempDir(s.fsconf.Temp, "update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(downloadDir)

	err = s.downloadIntegrations(version, downloadDir)
	if err != nil {
		return err
	}

	err = s.updateIntegrations(version, downloadDir)
	if err != nil {
		return err
	}

	s.logger.Info("Downloaded integrations")
	return nil
}

// Update updates both the agent and integrations to the specified version.
func (s *Updater) Update(version string) error {
	err := os.MkdirAll(s.fsconf.Temp, 0777)
	if err != nil {
		return err
	}

	downloadDir, err := ioutil.TempDir(s.fsconf.Temp, "update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(downloadDir)

	_, err = s.downloadBinary("pinpoint-agent", version, downloadDir)
	if err != nil {
		return err
	}

	err = s.downloadIntegrations(version, downloadDir)
	if err != nil {
		return err
	}

	s.logger.Info("Replacing agent binary")
	err = s.updateAgent(version, downloadDir)
	if err != nil {
		return err
	}

	s.logger.Info("Replacing integration binaries")
	err = s.updateIntegrations(version, downloadDir)
	if err != nil {
		return fmt.Errorf("updateIntegrations: %v", err)
	}

	s.logger.Info("Updated both agent and integrations")
	return nil
}

const distBinaryName = "pinpoint-agent"

func (s *Updater) downloadIntegrations(version string, dir string) error {

	bins := build.BuiltinIntegrationBinaries()
	if len(bins) == 0 {
		return errors.New("not builtin integration binaries specified in ldflags")
	}

	integrationsDir := filepath.Join(dir, "integrations")
	err := os.MkdirAll(integrationsDir, 0777)
	if err != nil {
		return err
	}

	for _, bin := range bins {
		_, err := s.downloadBinary("integrations/"+bin, version, integrationsDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Updater) updateAgent(version, downloadDir string) error {
	loc, err := os.Executable()
	if err != nil {
		return err
	}
	repl := filepath.Join(downloadDir, distBinaryName)
	if runtime.GOOS == "windows" {
		repl += ".exe"
	}

	err = replaceRestoringIfFailed(loc, repl, s.fsconf.Temp)
	if err != nil {
		return fmt.Errorf("failed to replace agent: %v", err)
	}
	return nil
}

func (s *Updater) updateIntegrations(version string, downloadDir string) error {
	downloadedIntegrations := filepath.Join(downloadDir, "integrations")
	ok, err := fs.Exists(s.integrationsSubDir)
	if err != nil {
		return err
	}
	if !ok {
		// integration dir did not exist, create an empty one, so that we can use replaceRestoringIfFailed
		err = os.MkdirAll(s.integrationsSubDir, 0777)
		if err != nil {
			return fmt.Errorf("could not create integrations dir: %v", err)
		}
	}

	err = replaceRestoringIfFailed(s.integrationsSubDir, downloadedIntegrations, s.fsconf.Temp)
	if err != nil {
		return fmt.Errorf("failed to replace integrations: %v", err)
	}
	return nil
}

// on windows we will not be able to delete the current agent, because the main service process is running it. but the second backup name will work.
// retrying RemoveAll 2 times for this
func backupLoc(loc string) (backupLoc string, _ error) {
	i := 0
	var lastErr, busyError error
	for {
		backupLoc = loc + ".old" + strconv.Itoa(i)
		err := os.RemoveAll(backupLoc)
		if err == nil {
			return
		}
		if strings.Contains(err.Error(), "device or resource busy") {
			busyError = killBusyProcesses(err)
		}
		lastErr = err
		i++
		if i >= 2 {
			return "", fmt.Errorf("could not find name for backup, old backup can not be deleted, failed after %v attempts, last errors %v, %v ", i, lastErr, busyError)
		}
	}
}

func replaceRestoringIfFailed(loc string, repl string, tmpDir string) error {
	repl2 := loc + ".new"
	backup, err := backupLoc(loc)
	if err != nil {
		return err
	}

	// copy from loc to new to allow the files being on different drives, happens in make docker-dev
	err = os.RemoveAll(repl2)
	if err != nil {
		return err
	}
	err = fs.Copy(repl, repl2)
	if err != nil {
		return fmt.Errorf("could not copy new download, err: %v", err)
	}
	fi, err := os.Stat(repl2)
	if err != nil {
		return fmt.Errorf("could not stat download copy, err: %v", err)
	}
	if fi.IsDir() {
		err := fs.ChmodFilesInDir(repl2, 0777)
		if err != nil {
			return fmt.Errorf("could not chmod new binaries in dir, err: %v", err)
		}
	} else {
		err := os.Chmod(repl2, 0777)
		if err != nil {
			return fmt.Errorf("could not chmod new binary, err: %v", err)
		}
	}
	err = os.Rename(loc, backup)
	if err != nil {
		return fmt.Errorf("could not rename curr to backup, err: %v", err)
	}
	err = os.Rename(repl2, loc)
	if err != nil {
		return fmt.Errorf("could move new into place, err: %v", err)
	}
	if err != nil {
		// rename failed, restore prev
		err2 := os.Rename(backup, loc)
		if err2 != nil {
			return fmt.Errorf("failed to replace: %v and failed to restore: %v", err, err2)
		}
		return fmt.Errorf("failed to replace: %v", err)
	}
	return nil
}

func (s *Updater) downloadBinary(urlPath string, version string, tmpDir string) (loc string, rerr error) {
	platformArch := runtime.GOOS + "-" + runtime.GOARCH
	switch runtime.GOOS {
	case "windows", "linux":
	default:
		rerr = errors.New("platform not supported: " + platformArch)
		return
	}
	if runtime.GOARCH != "amd64" {
		rerr = errors.New("platform not supported: " + platformArch)
		return
	}

	s3BinariesPrefix := ""
	if os.Getenv("PP_AGENT_USE_DIRECT_UPDATE_URL") != "" {
		s3BinariesPrefix = "https://pinpoint-agent.s3.amazonaws.com/releases"
	} else {

		s3BinariesPrefix = pstrings.JoinURL(api.BackendURL(api.EventService, s.channel), "agent", "download")
	}

	url := pstrings.JoinURL(s3BinariesPrefix, version, "bin-gz", platformArch, urlPath)
	if runtime.GOOS == "windows" {
		url += ".exe"
	}
	url += ".gz"

	bin := path.Base(urlPath)

	s.logger.Info("downloading binary", "bin", bin, "url", url)

	resp, err := http.Get(url)
	if err != nil {
		rerr = err
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		rerr = fmt.Errorf("could not download binary, status code: %v url: %v", resp.StatusCode, url)
		return
	}

	loc = filepath.Join(tmpDir, bin)
	if runtime.GOOS == "windows" {
		loc += ".exe"
	}

	r, err := gzip.NewReader(resp.Body)
	if err != nil {
		rerr = err
		return
	}

	err = fs.WriteToTempAndRename(r, loc)
	if err != nil {
		rerr = err
		return
	}
	s.logger.Info("downloaded binary", "bin", bin)

	return
}
