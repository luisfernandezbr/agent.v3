// +build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func kill(process *os.Process) error {

	// This method works, but the problem is that it kills the process, it's children,
	// and its parents. So, even if you call this on a subprocess, the current process
	// will also be killed.
	//
	// The benefit of using this is that we can listen to the signal.Interrupt
	/*
		dll, err := windows.LoadDLL("kernel32.dll")
		if err != nil {
			return err
		}
		defer dll.Release()

		f, err := dll.FindProc("GenerateConsoleCtrlEvent")
		if err != nil {
			return err
		}

		r1, _, err := f.Call(windows.CTRL_BREAK_EVENT, uintptr(process.Pid))
		if r1 == 0 {
			return err
		}
		return nil
	*/

	// This is a better method since it kills the process and its children
	//
	// The downside is that we cannot listen to the signal
	return exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", process.Pid)).Run()
}
