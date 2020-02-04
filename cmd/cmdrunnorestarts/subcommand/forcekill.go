package subcommand

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	pps "github.com/mitchellh/go-ps"
)

func Kill(opts KillCmdOpts, process *os.Process) error {
	prs, _ := pps.Processes()
	pid := process.Pid
	array := []int{pid}

	this, _ := pps.FindProcess(pid)
	if this == nil {
		return nil
	}
	// uncomment for debug
	// var names []string
	// if this != nil {
	// 	names = []string{this.Executable()}
	// }
	var find func(int)
	find = func(pid int) {
		for _, process := range prs {
			if process.PPid() == pid {
				array = append([]int{process.Pid()}, array...)
				// names = append([]string{process.Executable()}, names...)
				// fmt.Println("------> ", process.Executable())
				find(process.Pid())
			}
		}
	}
	find(pid)
	// fmt.Println("------> KILLING IN THIS ORDER", names)

	// handle errors
	for _, p := range array {
		pr, _ := os.FindProcess(p)
		if runtime.GOOS == "windows" {
			if err := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", p)).Run(); err != nil {
				opts.PrintLog("error calling taskkill", "err", err)
			}
		} else {
			if err := pr.Signal(os.Interrupt); err != nil {
				opts.PrintLog("error calling Signal(os.Interrupt)", "err", err)
			}
		}
	}
	return nil
}
