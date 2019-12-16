// +build !windows

package interrupt

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
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

	pgid, err := syscall.Getpgid(process.Pid)
	if err != nil {
		return err
	}
	return syscall.Kill(-pgid, syscall.SIGINT)
}
