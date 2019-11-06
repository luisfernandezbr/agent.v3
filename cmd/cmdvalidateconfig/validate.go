package cmdvalidateconfig

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/gitclone"
	"github.com/pinpt/agent.next/pkg/iloader"
	"github.com/pinpt/agent.next/rpcdef"

	"github.com/pinpt/agent.next/cmd/cmdintegration"
)

type Opts struct {
	cmdintegration.Opts
	Output io.Writer
}

type AgentConfig = cmdintegration.AgentConfig
type Integration = cmdintegration.Integration

func Run(opts Opts) error {
	exp, err := newValidator(opts)
	if err != nil {
		return err
	}
	defer exp.Destroy()
	return nil
}

type validator struct {
	*cmdintegration.Command

	Opts Opts

	integration  *iloader.Integration
	exportConfig rpcdef.ExportConfig
}

func newValidator(opts Opts) (_ *validator, rerr error) {
	s := &validator{}
	if len(opts.Integrations) != 1 {
		panic("pass exactly 1 integration")
	}

	var err error
	s.Command, err = cmdintegration.NewCommand(opts.Opts)
	if err != nil {
		rerr = err
		return
	}
	s.Opts = opts

	err = s.SetupIntegrations(nil)
	if err != nil {
		err := s.outputErr(err)
		if err != nil {
			rerr = err
			return
		}
		return
	}

	integration := opts.Integrations[0]
	id, err := integration.ID()
	if err != nil {
		return nil, err
	}

	s.integration = s.Integrations[id]
	s.exportConfig = s.ExportConfigs[id]

	err = s.runValidateAndPrint()
	if err != nil {
		rerr = err
		return
	}

	return s, nil
}

type Result struct {
	Errors []string `json:"errors"`
	// Success is true if there are no errors. Useful when returning result as json to ensure that marshalling worked.
	Success bool `json:"success"`
}

func (s *validator) runValidateAndPrint() error {
	errs := s.runValidate()
	return s.output(errs)
}

func (s *validator) outputErr(err error) error {
	return s.output([]string{err.Error()})
}

func (s *validator) output(errs []string) error {
	res := Result{}
	res.Errors = errs

	if len(res.Errors) == 0 {
		res.Success = true
	}

	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = s.Opts.Output.Write(b)
	if err != nil {
		return err
	}

	s.Logger.Info("validate-config completed", "errors", res.Errors)

	// BUG: last log message is missing without this
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (s *validator) runValidate() (errs []string) {
	ctx := context.Background()
	client := s.integration.RPCClient()

	rerr := func(err error) {
		errs = append(errs, err.Error())
	}

	res0, err := client.ValidateConfig(ctx, s.exportConfig)
	if err != nil {
		_ = s.CloseOnlyIntegrationAndHandlePanic(s.integration)
		rerr(err)
		return
	}

	if res0.RepoURL != "" { // repo url is only set for git integrations
		err = s.cloneRepo(res0.RepoURL)
		if err != nil {
			rerr(err)
			return
		}
	}

	err = s.CloseOnlyIntegrationAndHandlePanic(s.integration)
	if err != nil {
		rerr(err)
		return
	}

	return res0.Errors
}

func (s *validator) cloneRepo(url string) error {
	urlWithoutCreds, err := urlWithoutCreds(url)
	if err != nil {
		return fmt.Errorf("url passed to git clone validation is not valid, err: %v", err)
	}

	s.Logger.Info("git clone validation start", "url", urlWithoutCreds)

	if err := s.cloneRepo2(url); err != nil {
		return fmt.Errorf("git clone validation failed. url: %v err: %v", urlWithoutCreds, err)
	}

	s.Logger.Info("git clone validation success", "url", urlWithoutCreds)
	return nil
}

func urlWithoutCreds(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	u.User = nil
	return u.String(), nil
}

func (s *validator) Destroy() {
}

func (s *validator) cloneRepo2(url string) error {

	tmpDir, err := ioutil.TempDir(s.Locs.Temp, "validate-repo-clone-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := exec.CommandContext(ctx, "git", "clone", "--progress", url, tmpDir)
	var outBuf bytes.Buffer
	c.Stdout = &outBuf
	stderr, _ := c.StderrPipe()

	err = c.Start()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(stderr)
	var output []byte
	for scanner.Scan() {
		output = append(output, scanner.Bytes()...)

		line := scanner.Text()

		if strings.Contains(line, "Receiving objects:") ||
			strings.Contains(line, "Counting objects:") ||
			strings.Contains(line, "Enumerating objects:") ||
			strings.Contains(line, "You appear to have cloned an empty repository") {
			// If we see one of these lines, it means credentials are valid

			return nil
		}

	}

	if err := scanner.Err(); err != nil {
		return err
	}

	outputStr, err := gitclone.RedactCredsInText(string(output), url)
	if err != nil {
		return err
	}

	return errors.New(outputStr)
}
