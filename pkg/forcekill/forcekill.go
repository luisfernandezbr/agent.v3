// +build !windows

package forcekill

import (
	"fmt"
	"os"
	"os/exec"
)

var listening = false

func Kill(process *os.Process) error {
	return exec.Command("pkill", "-P", fmt.Sprintf("%d", process.Pid)).Run()
}
