package cmdservicerun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/pservice"
	"github.com/pinpt/go-common/log"
)

func Run(logger hclog.Logger) error {
	fmt.Println("service-run")

	for {
		execExport()
		<-time.After(time.Hour)
	}
}

func execExport() error {
	args := []string{
		"export",
	}
	return execSubcommand(args)
}

func execSubcommand(args []string) error {
	logger := log.NewNoOpTestLogger()
	runFn := func(ctx context.Context) error {

		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	delayFn := pservice.ExpRetryDelayFn(time.Minute, 15*time.Minute, 2)
	runRetrying := pservice.Retrying(logger, runFn, delayFn)
	return runRetrying(context.Background())
}
