// +build !windows

package sysinfo

import (
	"os/exec"
	"strings"
	"syscall"
)

func getFreeSpace(wd string) uint64 {
	var stat syscall.Statfs_t

	syscall.Statfs(wd, &stat)

	// Available blocks * size per block = available space in bytes
	return stat.Bavail * uint64(stat.Bsize)
}

func getAvailablePath() string {
	cmd := exec.Command("/bin/sh", "-c", "df -h")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return ""
	}

	var path string
	var higher, size uint64

	for _, line := range lines[1 : len(lines)-1] {
		if arr := strings.Split(line, "%"); len(arr) > 1 {
			str := arr[len(arr)-1]
			str = strings.TrimLeft(str, " ")
			size = getFreeSpace(str)

			if size > higher {
				higher = size
				path = str
			}
		}
	}
	return path
}
