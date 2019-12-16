// +build windows

package interrupt

import (
	"fmt"
	"os"
	"os/signal"

	"golang.org/x/sys/windows"
)

var listening = false

func Kill(process *os.Process) error {

	if !listening {
		listening = true
		ctrlc := make(chan os.Signal, 1)
		signal.Notify(ctrlc, os.Interrupt)

		go func() {
			select {
			case s := <-ctrlc:
				fmt.Println("control c catched", s)
				signal.Reset(os.Interrupt)
			}
		}()
	}

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
}
