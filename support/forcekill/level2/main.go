package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"
)

const name = "+----[level2]"

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
	started := time.Now()
	fmt.Println(name, "started")

	exitIfInterrupt()

	time.Sleep(50 * time.Second)
	// If killed by ../main.go, this will never finish - as intended
	fmt.Println(name, "exited normally", "duration", time.Since(started))
}
