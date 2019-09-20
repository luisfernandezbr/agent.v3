package cmdservicerun

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/pinpt/agent.next/cmd/cmdvalidateconfig"
)

func depointer(data map[string]interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var res map[string]interface{}
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *runner) validate(ctx context.Context, name string, config map[string]interface{}) (res cmdvalidateconfig.Result, _ error) {
	s.logger.Info("validating config for integration", "name", name)
	// convert to non pointer strings
	config, err := depointer(config)
	if err != nil {
		return res, err
	}
	inConf, name2, err := convertConfig(name, config, []string{})
	if err != nil {
		return res, err
	}
	in := cmdvalidateconfig.Integration{}
	in.Name = name2
	in.Config = inConf

	integrations := []cmdvalidateconfig.Integration{in}

	args := []string{"validate-config"}

	fs, err := newFsPassedParams(s.fsconf.Temp, []kv{
		{"--agent-config-file", s.agentConfig},
		{"--integrations-file", integrations},
	})
	if err != nil {
		return res, err
	}
	args = append(args, fs.Args()...)
	defer fs.Clean()

	err = s.runCommand(ctx, &res, args)
	s.logger.Info("validation completed", "success", res.Success)

	if err != nil {
		return res, err
	}
	return res, nil
}

func (s *runner) runCommand(ctx context.Context, res interface{}, args []string) error {
	rerr := func(err error) error {
		return fmt.Errorf("could not run subcommand %v err: %v", args[0], err)
	}

	err := os.MkdirAll(s.fsconf.Temp, 0777)
	if err != nil {
		return err
	}
	f, err := ioutil.TempFile(s.fsconf.Temp, "")
	if err != nil {
		return err
	}
	out := f.Name()
	f.Close()
	defer os.Remove(out)

	args = append(args, "--output-file", out)
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return rerr(err)
	}

	b, err := ioutil.ReadFile(out)
	if err != nil {
		return rerr(err)
	}

	err = json.Unmarshal(b, res)
	if err != nil {
		return rerr(fmt.Errorf("invalid data returned in command output, expecting json, err %v", err))
	}

	return nil
}
