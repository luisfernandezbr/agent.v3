// +build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func kill(process *os.Process) error {
	// The problem with process.Kill() is that it does not bubble to it's children
	//
	// This is a better method, it works in all unix systems
	return exec.Command("pkill", "-P", fmt.Sprintf("%d", process.Pid)).Run()
}
