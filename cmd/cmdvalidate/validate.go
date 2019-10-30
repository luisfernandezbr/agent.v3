package cmdvalidate

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/sysinfo"
	"github.com/pinpt/go-common/number"
)

// TODO: root is passed but not used, this is likely to break with custom --pinpoint flag passed
func Run(ctx context.Context, logger hclog.Logger, root string) (validate bool, err error) {

	const GB = 1024 * 1024 * 1024

	const MINIMUM_MEMORY = 16 * GB
	const MINIMUM_SPACE = 100 * GB
	const MINIMUM_NUM_CPU = 2 * 10
	const MININUM_GIT_VERSION = "2.13.0"

	// TODO: it should validate git executable as well, currently does not fail the checks if can't run the version
	// also can use c.Output() instead of run for easier code, the below can be simplified
	c := exec.CommandContext(ctx, "git", "version")
	var stdout, stderr bytes.Buffer
	var skipGit bool
	var currentGitVersion string
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err = c.Run(); err != nil {
		if strings.Contains(err.Error(), "found") || strings.Contains(err.Error(), "not recognized") {
			skipGit = true
			logger.Info("git", "msg", err.Error())
			err = nil
		} else {
			return
		}
	} else if stderr.String() != "" {
		err = fmt.Errorf("cmd err %s", stderr.String())
		return
	} else {
		currentGitVersion = strings.Trim(strings.Split(stdout.String(), " ")[2], "\n")
	}

	sysInfo := sysinfo.GetSystemInfo()

	val := validator{
		logger:  logger,
		isValid: true,
	}

	if sysInfo.TotalMemory < MINIMUM_MEMORY {
		val.invalid("memory", number.ToBytesSize(int64(sysInfo.TotalMemory)), number.ToBytesSize(int64(MINIMUM_MEMORY)))
	}
	if sysInfo.FreeSpace < MINIMUM_SPACE {
		val.invalid("space", number.ToBytesSize(int64(sysInfo.FreeSpace)), number.ToBytesSize(int64(MINIMUM_SPACE)))
		logger.Info("using pinpoint root", "dir", "TODO")
	}
	if sysInfo.NumCPU < MINIMUM_NUM_CPU {
		val.invalid("cpus", strconv.FormatInt(int64(sysInfo.NumCPU), 10), strconv.FormatInt(MINIMUM_NUM_CPU, 10))
	}
	// TODO: check minimum version using semver. otherwise may be wrong with 2.10 < 2.9
	if !skipGit && currentGitVersion < MININUM_GIT_VERSION {
		val.invalid("git version", currentGitVersion, MININUM_GIT_VERSION)
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
