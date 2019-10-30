package cmdvalidateconfig

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

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

	integrationName := opts.Integrations[0].Name
	s.integration = s.Integrations[integrationName]
	s.exportConfig = s.ExportConfigs[integrationName]

	err = s.runValidateAndPrint()
	if err != nil {
		rerr = err
		return
	}

	return s, nil
}

type Result struct {
	rpcdef.ValidationResult
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

	s.Logger.Debug("validate len repos", "len", len(res0.ReposURLs))

	for _, repoURL := range res0.ReposURLs {
		urlWithoutCreds, err := urlWithoutCreds(repoURL)
		if err != nil {
			rerr(fmt.Errorf("url passed to git clone validation is not valid, err: %v", err))
		}
		if err := s.testGitClone(repoURL); err != nil {
			rerr(fmt.Errorf("git clone validation failed. repoURL: %v err: %v", urlWithoutCreds, err))
			return
		}
		s.Logger.Info("git clone validation success", "repoURL", urlWithoutCreds)
		break
	}

	err = s.CloseOnlyIntegrationAndHandlePanic(s.integration)
	if err != nil {
		rerr(err)
		return
	}

	return res0.Errors
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

func (s *validator) testGitClone(repoURL string) (err error) {

	tmpDir, err := ioutil.TempDir(s.Locs.Temp, "validate-repo-clone-")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir)

	c := exec.Command("git", "clone", "--progress", repoURL, tmpDir)
	var outBuf bytes.Buffer
	c.Stdout = &outBuf
	stderr, _ := c.StderrPipe()

	done := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Split(bufio.ScanLines)
		output := ""
		for scanner.Scan() {
			m := scanner.Text()
			output += m
			if strings.Contains(m, "Receiving objects:") ||
				strings.Contains(m, "Counting objects:") ||
				strings.Contains(m, "Enumerating objects:") ||
				strings.Contains(m, "You appear to have cloned an empty repository") { // If we see one of these text, it means credentials are valid
				done <- nil
				return
			}

		}
		if err := scanner.Err(); err != nil {
			done <- err
			return
		}

		done <- fmt.Errorf(output)
	}()

	if err = c.Start(); err != nil { // we use start because we don't need the command to finish
		return
	}

	err = <-done

	return
}
