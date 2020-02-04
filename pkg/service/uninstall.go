package service

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/kardianos/service"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/subcommand"
	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/fsconf"
)

type UninstallOpts struct {
	PrintLog func(msg string, args ...interface{})
}

func UninstallAndDelete(opts UninstallOpts, pinpointRoot string) error {

	opts.PrintLog("deleting folders")

	fsconf := fsconf.New(pinpointRoot)

	conf, err := agentconf.Load(fsconf.Config2)
	if err != nil {
		return err
	}

	commands := []string{"export", "export-onboard-data", "validate-config"}

	for _, cmdname := range commands {
		if err := subcommand.KillCommand(subcommand.KillCmdOpts{
			PrintLog: func(msg string, args ...interface{}) {
				opts.PrintLog(msg, args)
			},
		}, cmdname); err != nil {
			return fmt.Errorf("error killing %s, err = %s", cmdname, err)
		}
	}

	integrationDir := conf.IntegrationsDir
	if integrationDir == "" {
		integrationDir = fsconf.IntegrationsDefaultDir
	}

	deleteEntity := func(location string, label string) error {
		err := os.RemoveAll(location)
		if err != nil {
			return fmt.Errorf("error deleting %s, error = %s", label, err)
		}
		return nil
	}

	items := []struct {
		location string
		label    string
	}{
		{
			fsconf.State,
			"state",
		},
		{
			integrationDir,
			"integrations",
		},
		{
			fsconf.Logs,
			"logs",
		},
		{
			fsconf.Cache,
			"cache",
		},
		{
			fsconf.Config2,
			"config",
		},
		{
			fsconf.Temp,
			"temp",
		},
	}

	for _, v := range items {
		if err := deleteEntity(v.location, v.label); err != nil && v.location != "" {
			return err
		}
		opts.PrintLog("deleted folder ", "label ", v.label, " folder ", v.location)
	}

	if !service.Interactive() {
		opts.PrintLog("uninstall service")

		cmd := exec.Command(os.Args[0], "service-uninstall")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			opts.PrintLog("error on service-uninstall", "err", err)
			return err
		}

		opts.PrintLog("service uninstalled")
	}

	return nil
}
