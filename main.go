package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent2/rpcdef"
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

type agentDelegate struct {
}

func (s agentDelegate) SendExported(modelType string, objs []rpcdef.ExportObj) {
	fmt.Println("agent: SendExported received event", modelType, "len(objs)=", len(objs))
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Output:     os.Stdout,
		Level:      hclog.Info,
		JSONFormat: false,
	})

	client := plugin.NewClient(&plugin.ClientConfig{
		Logger:          logger,
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         rpcdef.PluginMap,
		Cmd:             devIntegrationCommand("mock"),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}

	raw, err := rpcClient.Dispense("integration")
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}

	delegate := agentDelegate{}

	impl := raw.(rpcdef.Integration)
	ctx := context.Background()

	impl.Init(delegate)

	err = impl.Export(ctx)
	fmt.Println("call err", err)
}
