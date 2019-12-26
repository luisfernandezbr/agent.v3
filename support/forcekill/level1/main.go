package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"
)

const name = "+--[level1]"

func exitIfInterrupt() {
	// This does not work in windows with "taskkill"
	// But it works with Interrupt, leaving it here as a reference
	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	go func() {
		select {
		case s := <-sig:
			fmt.Println(name, "killed from external source", s)
			os.Exit(123)
		}
	}()
}

func main() {
	fmt.Println(name, "started")

	exitIfInterrupt()

	// Run ../level2/main.go
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	path = filepath.Join(path, "level2")

	cmd := exec.Command(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// If killed by ../main.go, this will never finish - as intended
	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok && exiterr.ExitCode() != 123 {
			fmt.Println(name, "level2 exit err", exiterr.String())
		}
	}

	time.Sleep(1 * time.Second)

	fmt.Println(name, "exited normally")
}
