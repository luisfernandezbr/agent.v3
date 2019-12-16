// +build !windows

package interrupt

import (
	"os"
	"syscall"
)

func Kill(process *os.Process) error {
	pgid, err := syscall.Getpgid(process.Pid)
	if err != nil {
		return err
	}
	return syscall.Kill(-pgid, syscall.SIGINT)
}
