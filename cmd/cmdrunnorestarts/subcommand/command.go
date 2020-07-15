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
	"github.com/pinpt/go-common/v10/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent/cmd/cmdintegration"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/logsender"
	"github.com/pinpt/agent/pkg/aevent"
	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/deviceinfo"
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
	Integrations      []inconfig.IntegrationAgent
	DeviceInfo        deviceinfo.CommonInfo
}

// Command is struct for executing cmdintegration based commands
type Command struct {
	logger       hclog.Logger
	tmpdir       string
	config       cmdintegration.AgentConfig
	agentConfig  agentconf.Config
	integrations []inconfig.IntegrationAgent
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

type KillCmdOpts struct {
	PrintLog func(msg string, args ...interface{})
}

// KillCommand stops a running process
func KillCommand(opts KillCmdOpts, cmdname string) error {
	opts.PrintLog("killing command manually", "cmd", cmdname)
	return removeProcess(opts, cmdname)
}

// Run executes the command
func (c *Command) Run(ctx context.Context, cmdname string, messageID string, res interface{}, args ...string) error {
	logFile, err := c.RunKeepLogFile(ctx, cmdname, messageID, res, args...)
	if logFile != "" {
		os.Remove(logFile)
	}
	if err != nil {
		return err
	}
	return nil
}

// RunKeepLogFile executes the command creating a file for stderr. It returns the path to that file.
func (c *Command) RunKeepLogFile(ctx context.Context, cmdname string, messageID string, res interface{}, args ...string) (logFileName string, rerrv error) {
	rerr := func(err error) {
		rerrv = fmt.Errorf("could not run subcommand %v err: %v", cmdname, err)
	}
	fs, err := newFsPassedParams(c.tmpdir, []kv{
		{"--agent-config-file", c.config},
		{"--integrations-file", c.integrations},
	})
	defer fs.Clean()
	if err != nil {
		rerr(err)
		return
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
		rerr(err)
		return
	}
	var outfile *os.File
	// the output goes into os.File, don't create the os.File if there is no res
	if res != nil {
		outfile, err = ioutil.TempFile(c.tmpdir, "")
		if err != nil {
			rerr(err)
			return
		}
		outfile.Close()
		defer os.Remove(outfile.Name())
		flags = append(flags, "--output-file", outfile.Name())
	}

	panicFile, err := ioutil.TempFile(c.tmpdir, "")
	if err != nil {
		rerr(err)
		return
	}
	defer panicFile.Close()
	defer os.Remove(panicFile.Name())

	logFile, err := ioutil.TempFile(c.tmpdir, "")
	if err != nil {
		rerr(err)
		return
	}
	defer logFile.Close()
	logFileName = logFile.Name()

	cmd := exec.CommandContext(ctx, os.Args[0], flags...)
	if messageID != "" {
		opts := logsender.Opts{}
		opts.Logger = c.logger
		opts.Conf = c.agentConfig
		opts.CmdName = cmdname
		opts.MessageID = messageID
		ls := logsender.New(opts)
		defer func() {
			err := ls.Close()
			if err != nil {
				c.logger.Error("could not send export logs to the server", "err", err)
			}
		}()
		cmd.Stdout = io.MultiWriter(os.Stdout, logFile, ls)
		cmd.Stderr = io.MultiWriter(os.Stderr, logFile, panicFile, ls)
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
		cmd.Stderr = io.MultiWriter(os.Stderr, logFile, panicFile)
	}

	if err := cmd.Start(); err != nil {
		rerr(fmt.Errorf("could not start the sub command: %v", err))
		return
	}

	if cmdname == "export" { // for now, only allow this command to be cancelled
		if err := addProcess(c.logger, cmdname, cmd.Process); err != nil {
			rerr(fmt.Errorf("could not start the sub command, process already running: %v %v", err, cmdname))
			return
		}
		defer func() {
			// ignore error in this case since it will return an error if the process was kill manually
			opts := KillCmdOpts{
				PrintLog: func(msg string, args ...interface{}) {
					c.logger.Debug(msg, args)
				},
			}
			removeProcess(opts, cmdname)
		}()
	}

	err = cmd.Wait()

	if err != nil {
		if cmdname == "export" {
			if _, ok := processes[cmdname]; !ok {
				rerrv = &Cancelled{s: cmdname + " cancelled"}
				return
			}
		}
	}

	if err != nil {
		// the command wont be in the processes map if it has been canceled
		if err := panicFile.Close(); err != nil {
			rerr(fmt.Errorf("could not close stderr file: %v", err))
			return
		}
		err2 := c.handlePanic(panicFile.Name(), cmdname)
		if err2 != nil {
			rerr(fmt.Errorf("could not get crash report file: %v command failed with: %v", err2, err))
			return
		}
		rerr(fmt.Errorf("run: %v", err))
		return
	}

	if res != nil {
		b, err := ioutil.ReadFile(outfile.Name())
		if err != nil {
			rerr(err)
			return
		}
		//c.logger.Debug("got output from a subcommand", "v", string(b))
		err = json.Unmarshal(b, res)
		if err != nil {
			rerr(fmt.Errorf("invalid data returned in command output, expecting json, err %v", err))
			return
		}
	}

	return
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
	if err := aevent.Publish(context.Background(), publishEvent, c.agentConfig.Channel, c.agentConfig.APIKey); err != nil {
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
func removeProcess(opts KillCmdOpts, name string) error {
	if p, o := processes[name]; o {
		opts.PrintLog("removing process from map", "name", name, "pid", fmt.Sprint(p.Pid))
		delete(processes, name)
		Kill(opts, p)
	}

	return nil
}
