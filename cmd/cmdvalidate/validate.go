package cmdvalidate

import (
	"context"
	"fmt"
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
	const MINIMUM_SPACE = 100 * GB
	const MINIMUM_NUM_CPU = 2
	const MININUM_GIT_VERSION = "2.13.0"

	c := exec.CommandContext(ctx, "git", "version")
	var skipGit bool
	var currentGitVersion string
	var bts []byte
	if bts, err = c.Output(); err != nil {
		skipGit = true
		logger.Info("git", "msg", err.Error())
		err = nil
	} else {
		currentGitVersion = strings.Trim(strings.Split(string(bts), " ")[2], "\n")
	}

	sysInfo := sysinfo.GetSystemInfo(root)

	val := validator{
		logger:  logger,
		isValid: true,
	}

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
		cv, _ := semver.New(currentGitVersion)
		mv, _ := semver.New(MININUM_GIT_VERSION)
		if cv.LT(*mv) {
			val.invalid("git version", currentGitVersion, MININUM_GIT_VERSION)
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
