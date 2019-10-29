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
)

func Run(ctx context.Context, logger hclog.Logger, terminal bool) (validate bool, err error) {

	const MINIMUM_MEMORY = 17179869184
	const MINIMUM_SPACE = 107374182400
	const MINIMUM_NUM_CPU = 2
	const MININUM_GIT_VERSION = "2.13.0"

	pm := printMsg{
		terminal: terminal,
		logger:   logger,
	}

	c := exec.CommandContext(ctx, "git", "version")
	var stdout, stderr bytes.Buffer
	var skipGit bool
	var currentGitVersion string
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err = c.Run(); err != nil {
		if strings.Contains(err.Error(), "found") {
			skipGit = true
			if terminal {
				fmt.Println(strings.TrimLeft(err.Error(), "exec: "))
			} else {
				logger.Info("git", "msg", err.Error())
			}
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

	if sysInfo.Memory < MINIMUM_MEMORY {
		pm.printMsg("memory", formatSize(sysInfo.Memory), "16Gb")
	}
	if sysInfo.FreeSpace < MINIMUM_SPACE {
		pm.printMsg("space", formatSize(sysInfo.FreeSpace), "100Gb")
	}
	if sysInfo.NumCPU < MINIMUM_NUM_CPU {
		pm.printMsg("cpus", strconv.FormatInt(int64(sysInfo.NumCPU), 10), strconv.FormatInt(MINIMUM_NUM_CPU, 10))
	}
	if !skipGit && currentGitVersion < MININUM_GIT_VERSION {
		pm.printMsg("git version", currentGitVersion, MININUM_GIT_VERSION)
	}
	if !pm.validate {
		return
	}

	return true, nil
}

type printMsg struct {
	terminal bool
	logger   hclog.Logger
	validate bool
}

func (p *printMsg) printMsg(label, actual, expected string) {
	msg := fmt.Sprintf("%s available %s. %s is needed\n", label, actual, expected)
	p.validate = false
	if p.terminal {
		fmt.Printf(msg)
	} else {
		p.logger.Info("validate", msg)
	}
}

func formatSize(size uint64) string {
	switch {
	case size < 1024:
		s := strconv.FormatUint(size, 10)
		return s + "b"
	case size < 1048576:
		s := strconv.FormatUint(size/1024, 10)
		return s + "Kb"
	case size < 1073741824:
		s := strconv.FormatUint(size/1048576, 10)
		return s + "Mb"
	case size < 1099511627776:
		s := strconv.FormatUint(size/1073741824, 10)
		return s + "Gb"
	}
	return "-1"
}
