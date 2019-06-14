package cmdexport

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func devIntegrationCommand(integrationName string) *exec.Cmd {
	// build to catch compile errors
	// we don't need the resulting binary
	cmd := exec.Command("go", "build", "-o", filepath.Join(os.TempDir(), "out"), "github.com/pinpt/agent2/integrations/"+integrationName)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return exec.Command("go", "run", "github.com/pinpt/agent2/integrations/"+integrationName)
}
