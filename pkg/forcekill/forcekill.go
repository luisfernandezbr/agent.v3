// +build !windows

package forcekill

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	pps "github.com/mitchellh/go-ps"
)

func Kill(process *os.Process) error {
	prs, _ := pps.Processes()
	pid := process.Pid
	array := []int{pid}

	// uncomment for debug
	// this, _ := pps.FindProcess(pid)
	// var names []string
	// if this != nil {
	// 	names = []string{this.Executable()}
	// }

	for {
		var found bool
		for _, p := range prs {
			if p.PPid() == pid {
				array = append([]int{p.Pid()}, array...)
				// names = append([]string{p.Executable()}, names...)
				pid = p.Pid()
				found = true
				break
			}
		}
		if found == false {
			break
		}
	}
	// fmt.Println("KILLING IN THIS ORDER", names)
	for _, p := range array {
		pr, _ := os.FindProcess(p)
		if runtime.GOOS == "windows" {
			exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", p)).Run()
		} else {
			pr.Signal(os.Interrupt)
		}
	}
	return nil
}
