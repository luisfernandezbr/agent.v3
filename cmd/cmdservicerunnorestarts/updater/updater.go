package updater

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/build"
	"github.com/pinpt/agent.next/pkg/fs"
	"github.com/pinpt/agent.next/pkg/fsconf"
)

const s3BinariesPrefix = "https://pinpoint-agent.s3.amazonaws.com/releases"

type Updater struct {
	logger hclog.Logger
	fsconf fsconf.Locs
}

func New(logger hclog.Logger, fsconf fsconf.Locs) *Updater {
	return &Updater{
		logger: logger,
		fsconf: fsconf,
	}
}

func (s *Updater) DownloadIntegrationsIfMissing() error {
	dir := s.fsconf.Integrations
	exists, err := fs.Exists(dir)
	if err != nil {
		return fmt.Errorf("Could not read integration dir: %v", err)
	}
	if exists {
		return nil
	}

	version := os.Getenv("PP_AGENT_VERSION")

	s.logger.Info("Integrations dir does not exist, downloading integrations", "dir", dir, "version", version)

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
		return err
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
	ok, err := fs.Exists(s.fsconf.Integrations)
	if err != nil {
		return err
	}
	if !ok {
		// integration dir did not exist, create an empty one, so that we can use replaceRestoringIfFailed
		err = os.MkdirAll(s.fsconf.Integrations, 0777)
		if err != nil {
			return err
		}
	}

	err = replaceRestoringIfFailed(s.fsconf.Integrations, downloadedIntegrations, s.fsconf.Temp)
	if err != nil {
		return fmt.Errorf("failed to replace integrations: %v", err)
	}
	return nil
}

func replaceRestoringIfFailed(loc string, repl string, tmpDir string) error {
	repl2 := loc + ".new"
	backup := loc + ".old"

	err := os.RemoveAll(backup)
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
	platform := runtime.GOOS
	switch platform {
	case "windows", "linux":
	case "darwin":
		platform = "macos"
	default:
		rerr = errors.New("platform not supported: " + platform)
		return
	}

	url := s3BinariesPrefix + "/" + version + "/" + platform + "/" + urlPath
	if platform == "windows" {
		url += ".exe"
	}

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
	if platform == "windows" {
		loc += ".exe"
	}
	err = fs.WriteToTempAndRename(resp.Body, loc)
	if err != nil {
		rerr = err
		return
	}
	s.logger.Info("downloaded binary", "bin", bin)

	return
}
