// Package subcommand provides a way to execute subcommands
// while passing the arguments and output via filesystem
// to avoid data size limitations.
// It also shares the configuration and other params needed
// to run cmdintegration and handles panics.
package subcommand

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent/cmd/cmdintegration"
	"github.com/pinpt/agent/cmd/cmdservicerunnorestarts/logsender"
	"github.com/pinpt/agent/cmd/cmdvalidateconfig"
	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/deviceinfo"
	"github.com/pinpt/agent/pkg/forcekill"
)

// Cancelled implementation of error.
type Cancelled struct {
	s string
}

func (e *Cancelled) Error() string {
	return e.s
}

// Opts are options needed to create Command
type Opts struct {
	Logger            hclog.Logger
	Tmpdir            string
	IntegrationConfig cmdintegration.AgentConfig
	AgentConfig       agentconf.Config
	Integrations      []cmdvalidateconfig.Integration
	DeviceInfo        deviceinfo.CommonInfo
}

// Command is struct for executing cmdintegration based commands
type Command struct {
	logger       hclog.Logger
	tmpdir       string
	config       cmdintegration.AgentConfig
	agentConfig  agentconf.Config
	integrations []cmdvalidateconfig.Integration
	deviceInfo   deviceinfo.CommonInfo
}

// New creates a command
func New(opts Opts) (*Command, error) {
	rerr := func(str string) (*Command, error) {
		return nil, errors.New("subcommand: initialization: " + str)
	}
	if opts.Logger == nil {
		return rerr(`opts.Logger == nil`)
	}
	if opts.Tmpdir == "" {
		return rerr(`opts.Tmpdir == ""`)
	}
	if opts.IntegrationConfig.PinpointRoot == "" {
		return rerr(`opts.IntegrationConfig.PinpointRoot == ""`)
	}
	if opts.AgentConfig.DeviceID == "" {
		return rerr(`opts.AgentConfig.DeviceID == ""`)
	}
	if opts.Integrations == nil {
		return rerr(`opts.Integrations == nil`)
	}
	if opts.DeviceInfo.CustomerID == "" {
		return rerr(`opts.DeviceInfo.CustomerID == ""`)
	}
	s := &Command{}
	s.logger = opts.Logger
	s.tmpdir = opts.Tmpdir
	s.config = opts.IntegrationConfig
	s.agentConfig = opts.AgentConfig
	s.integrations = opts.Integrations
	s.deviceInfo = opts.DeviceInfo
	return s, nil
}

// KillCommand stops a running process
func KillCommand(logger hclog.Logger, cmdname string) error {
	if cmdname != "export" {
		return errors.New("only supported for export command")
	}
	logger.Debug("killing command manually", "cmd", cmdname)
	return removeProcess(logger, cmdname)
}

// Run executes the command
func (c *Command) Run(ctx context.Context, cmdname string, messageID string, res interface{}, args ...string) error {

	rerr := func(err error) error {
		return fmt.Errorf("could not run subcommand %v err: %v", cmdname, err)
	}

	fs, err := newFsPassedParams(c.tmpdir, []kv{
		{"--agent-config-file", c.config},
		{"--integrations-file", c.integrations},
	})
	defer fs.Clean()
	if err != nil {
		return rerr(err)
	}
	flags := append([]string{cmdname}, fs.Args()...)
	flags = append(flags, "--log-format", "json")
	if args != nil {
		flags = append(flags, args...)
	}
	// This shouldn't be necessary, we already pass it in --agent-config-file
	// TODO: check why the value from --agent-config-file is not used
	flags = append(flags, "--pinpoint-root="+c.config.PinpointRoot)

	if err := os.MkdirAll(c.tmpdir, 0777); err != nil {
		return rerr(err)
	}
	var outfile *os.File
	// the output goes into os.File, don't create the os.File if there is no res
	if res != nil {
		outfile, err = ioutil.TempFile(c.tmpdir, "")
		if err != nil {
			return err
		}
		outfile.Close()
		defer os.Remove(outfile.Name())
		flags = append(flags, "--output-file", outfile.Name())
	}

	logsfile, err := ioutil.TempFile(c.tmpdir, "")
	if err != nil {
		return rerr(err)
	}
	defer logsfile.Close()
	defer os.Remove(logsfile.Name())

	cmd := exec.CommandContext(ctx, os.Args[0], flags...)
	if messageID != "" {
		ls := logsender.New(c.logger, c.agentConfig, cmdname, messageID)
		defer func() {
			err := ls.Close()
			if err != nil {
				c.logger.Error("could not send export logs to the server", "err", err)
			}
		}()

		cmd.Stdout = io.MultiWriter(os.Stdout, ls)
		cmd.Stderr = io.MultiWriter(os.Stderr, logsfile, ls)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.MultiWriter(os.Stderr, logsfile)
	}

	if err := cmd.Start(); err != nil {
		return rerr(fmt.Errorf("could not start the sub command: %v", err))
	}
	if cmdname == "export" { // for now, only allow this command to be cancelled
		if err := addProcess(c.logger, cmdname, cmd.Process); err != nil {
			return rerr(fmt.Errorf("could not start the sub command, process already running: %v %v", err, cmdname))
		}
		defer func() {
			// ignore error in this case since it will return an error if the process was kill manually
			removeProcess(c.logger, cmdname)
		}()
	}
	err = cmd.Wait()

	if err != nil && cmdname == "export" {
		if _, ok := processes[cmdname]; !ok {
			return &Cancelled{s: "export cancelled"}
		}
	}

	if err != nil {
		// the command wont be in the processes map if it has been canceled
		if err := logsfile.Close(); err != nil {
			return rerr(fmt.Errorf("could not close stderr file: %v", err))
		}
		err2 := c.handlePanic(logsfile.Name(), cmdname)
		if err2 != nil {
			return rerr(fmt.Errorf("could not get crash report file: %v command failed with: %v", err2, err))
		}
		return rerr(fmt.Errorf("run: %v", err))
	}

	if res != nil {
		b, err := ioutil.ReadFile(outfile.Name())
		if err != nil {
			return rerr(err)
		}
		err = json.Unmarshal(b, res)
		if err != nil {
			return rerr(fmt.Errorf("invalid data returned in command output, expecting json, err %v", err))
		}
	}
	return nil
}

func (c *Command) handlePanic(filename, cmdname string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	msg := string(b)
	c.logger.Info("Sub-command crashed will send to backend", "cmd", cmdname)
	data := &agent.Crash{
		Data:      &msg,
		Type:      agent.CrashTypeCrash,
		Component: cmdname,
	}
	date.ConvertToModel(time.Now(), &data.CrashDate)
	c.deviceInfo.AppendCommonInfo(data)
	publishEvent := event.PublishEvent{
		Object: data,
		Headers: map[string]string{
			"uuid":        c.deviceInfo.DeviceID,
			"customer_id": c.config.CustomerID,
			"job_id":      c.config.Backend.ExportJobID,
		},
	}
	if err := event.Publish(context.Background(), publishEvent, c.agentConfig.Channel, c.agentConfig.APIKey); err != nil {
		return fmt.Errorf("error sending agent.Crash to backend, err: %v", err)
	}
	return nil
}

type kv struct {
	K string
	V interface{}
}

type fsPassedParams struct {
	args    []kv
	tempDir string
	files   []string
}

func newFsPassedParams(tempDir string, args []kv) (*fsPassedParams, error) {
	s := &fsPassedParams{}
	s.args = args
	s.tempDir = tempDir
	for _, arg := range args {
		loc, err := s.writeFile(arg.V)
		if err != nil {
			return nil, err
		}
		s.files = append(s.files, loc)
	}
	return s, nil
}

func (s *fsPassedParams) writeFile(obj interface{}) (string, error) {
	err := os.MkdirAll(s.tempDir, 0777)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	f, err := ioutil.TempFile(s.tempDir, "")
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.Write(b)
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}

func (s *fsPassedParams) Args() (res []string) {
	for i, kv0 := range s.args {
		k := kv0.K
		v := s.files[i]
		res = append(res, k, v)
	}
	return
}

func (s *fsPassedParams) Clean() error {
	for _, f := range s.files {
		err := os.Remove(f)
		if err != nil {
			return err
		}
	}
	return nil
}

var processes map[string]*os.Process

func init() {
	processes = make(map[string]*os.Process)
}

// Warning! Not safe for concurrent use.
func addProcess(logger hclog.Logger, name string, p *os.Process) error {
	if _, o := processes[name]; o {
		return errors.New("process already exists: " + name)
	}
	logger.Debug("adding process to map", "name", name)
	processes[name] = p
	return nil
}

// Warning! Not safe for concurrent use.
func removeProcess(logger hclog.Logger, name string) error {
	if _, o := processes[name]; !o {
		return fmt.Errorf("process name '%s' not found in map", name)
	}
	logger.Debug("removing process from map", "name", name)
	p := processes[name]
	delete(processes, name)
	forcekill.Kill(p)
	return nil
}
