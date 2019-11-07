package cmdservicerun

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pinpt/agent.next/pkg/build"
	"github.com/pinpt/agent.next/pkg/fs"
)

const s3BinariesPrefix = "https://pinpoint-agent.s3.amazonaws.com/releases"

func (s *runner) downloadIntegrationsIfMissing() error {
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

	bins := build.BuiltinIntegrationBinaries()
	if len(bins) == 0 {
		return errors.New("not builtin integration binaries specified in ldflags")
	}

	err = os.MkdirAll(s.fsconf.Temp, 0777)
	if err != nil {
		return err
	}
	tmpIntegrations, err := ioutil.TempDir(s.fsconf.Temp, "integrations-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpIntegrations)

	for _, bin := range bins {
		err := s.downloadIntegration(bin, version, tmpIntegrations)
		if err != nil {
			return err
		}
	}

	return os.Rename(tmpIntegrations, s.fsconf.Integrations)
}

func (s *runner) downloadIntegration(bin string, version string, tmpDir string) error {
	platform := runtime.GOOS
	switch platform {
	case "windows", "linux":
	case "darwin":
		platform = "macos"
	default:
		return errors.New("platform not supported: " + platform)
	}

	url := s3BinariesPrefix + "/" + version + "/" + platform + "/integrations/" + bin
	if platform == "windows" {
		url += ".exe"
	}

	s.logger.Info("downloading integration", "bin", bin, "url", url)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	loc := filepath.Join(tmpDir, bin)
	if platform == "windows" {
		loc += ".exe"
	}
	err = fs.WriteToTempAndRename(resp.Body, loc)
	if err != nil {
		return err
	}
	s.logger.Info("downloaded integration", "bin", bin)

	return nil
}
