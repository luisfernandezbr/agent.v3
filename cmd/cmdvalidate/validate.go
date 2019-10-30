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

func Run(ctx context.Context, logger hclog.Logger, root string) (validate bool, err error) {

	const GIGA_BYTE_SIZE = 1024 * 1024 * 1024

	const MINIMUM_MEMORY = GIGA_BYTE_SIZE * 16
	const MINIMUM_SPACE = GIGA_BYTE_SIZE * 100
	const MINIMUM_NUM_CPU = 2
	const MININUM_GIT_VERSION = "2.13.0"

	pm := validator{
		logger:  logger,
		isValid: true,
	}

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

	if sysInfo.TotalMemory < MINIMUM_MEMORY {
		pm.validate("memory", number.ToBytesSize(int64(sysInfo.TotalMemory)), "16Gb")
	}
	if sysInfo.FreeSpace < MINIMUM_SPACE {
		pm.validate("space", number.ToBytesSize(int64(sysInfo.FreeSpace)), "100Gb")
	}
	if sysInfo.NumCPU < MINIMUM_NUM_CPU {
		pm.validate("cpus", strconv.FormatInt(int64(sysInfo.NumCPU), 10), strconv.FormatInt(MINIMUM_NUM_CPU, 10))
	}
	if !skipGit && currentGitVersion < MININUM_GIT_VERSION {
		pm.validate("git version", currentGitVersion, MININUM_GIT_VERSION)
	}
	if !pm.isValid {
		logger.Info("the minimum requirements were not met")
		return
	}

	logger.Info("correct minimum requirements")
	return true, nil
}

type validator struct {
	logger  hclog.Logger
	isValid bool
}

func (p *validator) validate(label, actual, expected string) {
	msg := fmt.Sprintf("%s available %s. %s is needed", label, actual, expected)
	p.isValid = false
	p.logger.Info("validate", "info", msg)
}
