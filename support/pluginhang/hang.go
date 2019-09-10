package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-ps"
)

// This is a test for a bug on windows where integrations processes do not terminate after the parent process exits.
// To test on windows
// make build
// Transfer the dist to windows
// .\test.exe
// .\test.exe check
func main() {

	processName := "aaihang"

	if len(os.Args) != 1 && os.Args[1] == "check" {
		processes, err := ps.Processes()
		if err != nil {
			panic(err)
		}
		for _, p := range processes {
			name := p.Executable()
			if strings.Contains(name, processName) {
				fmt.Printf("hanging process was not killed after parent exited, current running name: %v", name)
				os.Exit(1)
			}
			fmt.Println("All good. Process exited.")
			os.Exit(0)
		}
	}

	bins := os.Getenv("PP_AGENT_PLUGINHANG_BINS")
	if bins == "" {
		bins = "."
	}

	agentBin := filepath.Join(bins, "agent-next")

	cmd := exec.Command(agentBin, "export", "--agent-config-json", `{"customer_id":"c1", "dev_use_compiled_integrations":true}`, `--integrations-json`, `[{"name":"`+processName+`", "config":{"k":"v"}}]`, "--integrations-dir", bins)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
	}

}
