package main

import (
	"fmt"
	"os"

	pps "github.com/mitchellh/go-ps"
)

func kill(process *os.Process) error {

	prs, _ := pps.Processes()
	pid := process.Pid
	array := []int{pid}
	current, _ := pps.FindProcess(pid)
	names := []string{current.Executable()}

	for {
		var found bool
		for _, p := range prs {
			if p.PPid() == pid {
				array = append([]int{p.Pid()}, array...)
				names = append([]string{p.Executable()}, names...)
				pid = p.Pid()
				found = true
				break
			}
		}
		if found == false {
			break
		}
	}

	fmt.Println("KILLING IN THIS ORDER", names)
	for _, p := range array {
		pr, _ := os.FindProcess(p)
		pr.Signal(os.Interrupt)
	}
	return nil
}
