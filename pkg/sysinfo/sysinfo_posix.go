// +build !windows

package sysinfo

import (
	"syscall"
)

func getFreeSpace(wd string) uint64 {
	var stat syscall.Statfs_t

	syscall.Statfs(wd, &stat)

	// Available blocks * size per block = available space in bytes
	return stat.Bavail * uint64(stat.Bsize)
}
