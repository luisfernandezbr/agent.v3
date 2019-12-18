// +build windows

package forcekill

import (
	"fmt"
	"os"
	"os/exec"
)

var listening = false

func Kill(process *os.Process) error {
	return exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(process.Pid)).Run()
}
