package cmdvalidate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/blang/semver"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/sysinfo"
	"github.com/pinpt/go-common/number"
)

func Run(ctx context.Context, logger hclog.Logger, root string) (validate bool, err error) {

	const GB = 1024 * 1024 * 1024

	const MINIMUM_MEMORY = 16 * GB
	const MINIMUM_SPACE = 10 * GB
	const MINIMUM_NUM_CPU = 2
	const MINIMUM_GIT_VERSION = "2.13.0"

	val := validator{
		logger:  logger,
		isValid: true,
	}

	c := exec.CommandContext(ctx, "git", "version")
	var skipGit bool
	var currentGitVersion string
	var bts []byte
	if bts, err = c.Output(); err != nil {
		skipGit = true
		val.isValid = false
		logger.Error("git", "msg", err.Error())
		err = nil
	} else {
		currentGitVersion = string(bts)
	}

	err = os.MkdirAll(root, 0777)
	if err != nil {
		return false, err
	}
	sysInfo := sysinfo.GetSystemInfo(root)

	if sysInfo.TotalMemory < MINIMUM_MEMORY {
		val.invalid("memory", number.ToBytesSize(int64(sysInfo.TotalMemory)), number.ToBytesSize(int64(MINIMUM_MEMORY)))
	}
	if sysInfo.FreeSpace < MINIMUM_SPACE {
		val.invalid("space", number.ToBytesSize(int64(sysInfo.FreeSpace)), number.ToBytesSize(int64(MINIMUM_SPACE)))
		logger.Info("using pinpoint root", "dir", root)
	}
	if sysInfo.NumCPU < MINIMUM_NUM_CPU {
		val.invalid("cpus", strconv.FormatInt(int64(sysInfo.NumCPU), 10), strconv.FormatInt(MINIMUM_NUM_CPU, 10))
	}
	if !skipGit {
		ok, err := gitVersionGteq(currentGitVersion, MINIMUM_GIT_VERSION)
		if err != nil {
			logger.Error("can't parse git version", "err", err)
			val.invalid("git", currentGitVersion, MINIMUM_GIT_VERSION)
		}
		if !ok {
			val.invalid("git", currentGitVersion, MINIMUM_GIT_VERSION)
		}
	}

	if !val.isValid {
		logger.Error("Minimum system requirements were not met")
		return
	}

	logger.Info("Passed system requirement validation")
	return true, nil
}

type validator struct {
	logger  hclog.Logger
	isValid bool
}

func (p *validator) invalid(label, actual, expected string) {
	msg := fmt.Sprintf("%s available %s. required %s", label, actual, expected)
	p.isValid = false
	p.logger.Error(msg)
}

func gitVersionGteq(version string, min string) (bool, error) {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "git version ")
	parts := strings.Split(version, " ")
	if len(parts) != 0 {
		version = parts[0] // remove (Apple Git-117)
	}
	const win = ".windows."
	if strings.Contains(version, win) {
		p := strings.Index(version, win)
		version = version[0:p]
	}

	versionParsed, err := semver.New(version)
	if err != nil {
		return false, fmt.Errorf("git version format is not semver: %v", err)
	}
	minParsed, err := semver.New(min)
	if err != nil {
		return false, fmt.Errorf("min git version requirement has invalid format: %v", err)
	}
	return !versionParsed.LT(*minParsed), nil
}
