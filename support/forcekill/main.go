package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const name = "[main]"

func main() {

	fmt.Println(name, "main started")

	// Step 1:
	// Start a new process, ./level1/main.go
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	path = filepath.Join(path, "level1")

	cmd := exec.Command(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	process := cmd.Process

	// Step 2:
	// After a second, kill it!
	go func() {
		time.Sleep(2 * time.Second)
		kill(process)
	}()

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok && exiterr.ExitCode() != 123 {
			fmt.Println(name, "level1 exit err", exiterr.String())
		}
	}

	// If this process is killed, this will never run
	fmt.Println(name, "continue...")

	for i := 0; i < 5; i++ {
		fmt.Println(name, i+1)
		time.Sleep(1 * time.Second)
	}

}
