package cmdservicerunnorestarts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/pinpt/agent.next/cmd/cmdvalidateconfig"
	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/deviceinfo"
)

type subCommand struct {
	ctx             context.Context
	logger          hclog.Logger
	tmpdir          string
	config          cmdintegration.AgentConfig
	conf            agentconf.Config
	integrations    []cmdvalidateconfig.Integration
	integrationsDir string
	deviceInfo      deviceinfo.CommonInfo
	logWriter       *io.Writer
}

func (c *subCommand) validate() {
	if c.ctx == nil {
		panic("context is nil")
	}
	if c.logger == nil {
		panic("temp dir is nil")
	}
	if c.tmpdir == "" {
		panic("temp dir is nil")
	}
	if c.config.PinpointRoot == "" {
		panic("config is nil")
	}
	if c.conf.SystemID == "" {
		panic("conf is nil")
	}
	if c.integrations == nil {
		panic("integrations is nil")
	}
	if c.deviceInfo.SystemID == "" {
		panic("deviceInfo is nil")
	}
}

func (c *subCommand) err(arg string, err error) error {
	return fmt.Errorf("could not run subcommand %v err: %v", arg, err)
}

func (c *subCommand) run(cmdname string, res interface{}, args ...string) error {

	fs, err := newFsPassedParams(c.tmpdir, []kv{
		{"--agent-config-file", c.config},
		{"--integrations-file", c.integrations},
		{"--integrations-dir", c.integrationsDir},
	})
	defer fs.Clean()
	if err != nil {
		return err
	}
	flags := append([]string{cmdname}, fs.Args()...)
	flags = append(flags, "--log-format", "json")
	if args != nil {
		flags = append(flags, args...)
	}

	if err := os.MkdirAll(c.tmpdir, 0777); err != nil {
		return err
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
		return err
	}
	defer logsfile.Close()
	defer os.Remove(logsfile.Name())

	cmd := exec.CommandContext(c.ctx, os.Args[0], flags...)

	if c.logWriter != nil {
		cmd.Stdout = io.MultiWriter(os.Stdout, *c.logWriter, logsfile)
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = io.MultiWriter(os.Stderr, logsfile)

	if err := cmd.Run(); err != nil {
		logsfile.Close()
		if err2 := c.handlePanic(logsfile.Name(), cmdname); err2 != nil {
			return c.err(cmdname, fmt.Errorf("command err: %v. could not open the log file to save the crash report, err: %v", err, err2))
		}
		return c.err(cmdname, err)
	}
	logsfile.Close()

	if res != nil {
		b, err := ioutil.ReadFile(outfile.Name())
		if err != nil {
			return c.err(cmdname, err)
		}
		err = json.Unmarshal(b, res)
		if err != nil {
			return c.err(cmdname, fmt.Errorf("invalid data returned in command output, expecting json, err %v", err))
		}
	}
	return nil
}

func (c *subCommand) handlePanic(filename, cmdname string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	if len(b) > 0 {
		msg := string(b)
		c.logger.Info("crash detected!")
		if c.config.Backend.Enable {
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
			if err := event.Publish(context.Background(), publishEvent, c.conf.Channel, c.conf.APIKey); err != nil {
				return fmt.Errorf("error sending agent.Crash to backend, err: %v", err)
			}
			c.logger.Info("crash sent to backend")
		}
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
