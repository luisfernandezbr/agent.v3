package cmdrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/fs"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/pservice"
	"github.com/pinpt/agent/pkg/service"
)

type Opts struct {
	Logger          hclog.Logger
	PinpointRoot    string
	IntegrationsDir string
}

func Run(ctx context.Context, opts Opts, cancel chan bool) error {
	s, err := newRunner(opts)
	if err != nil {
		return err
	}
	return s.Run(cancel)
}

type runner struct {
	opts   Opts
	logger hclog.Logger
	fsconf fsconf.Locs
}

func newRunner(opts Opts) (*runner, error) {
	s := &runner{}
	s.opts = opts
	s.logger = opts.Logger
	s.fsconf = fsconf.New(opts.PinpointRoot)
	return s, nil
}

func (s *runner) Run(cancel chan bool) error {

	s.logger.Info("starting service-run-with-restarts", "pinpoint-root", s.opts.PinpointRoot, "integration-dir", s.opts.IntegrationsDir)

	delay := pservice.ExpRetryDelayFn(15*time.Second, 3*time.Hour, 2)
	resetFailuresAfter := 24 * time.Hour
	done, cancelPservice := pservice.AsyncRunBg(pservice.Retrying(s.logger, s.runService, delay, resetFailuresAfter))

	return s.CaptureShutdown(cancelPservice, cancel, done)
}

func (s *runner) CaptureShutdown(cancelPservice func(), cancelRunner chan bool, done chan error) error {
	cancelSignal := make(chan os.Signal, 1)
	signal.Notify(cancelSignal, os.Interrupt, syscall.SIGTERM)
	select {
	case <-cancelSignal:
		s.logger.Info("signal received exiting")
		if cancelRunner != nil {
			<-cancelRunner
		}
	case <-cancelRunner:
		s.logger.Info("cancel received from service control")
	case e := <-done:
		s.logger.Info("exited", "err", e)
		cancelPservice()
		return service.ErrUninstallExit
	}

	cancelPservice()
	<-done
	s.logger.Info("exited")
	return nil
}

func (s *runner) runService(ctx context.Context) error {
	fn := time.Now().UTC().Format(time.RFC3339Nano)
	fn = strings.ReplaceAll(fn, ":", "-")
	fn = strings.ReplaceAll(fn, ".", "-")
	fn += ".log"
	errFileLoc := filepath.Join(s.fsconf.ServiceRunCrashes, fn)

	err := os.MkdirAll(filepath.Dir(errFileLoc), 0777)
	if err != nil {
		return fmt.Errorf("could not create dir for err output: %v", err)
	}
	errFile, err := os.Create(errFileLoc)
	if err != nil {
		return fmt.Errorf("could not create file for err output: %v", err)
	}
	defer errFile.Close()
	stderr := io.MultiWriter(os.Stderr, errFile)

	cmd := exec.CommandContext(ctx, os.Args[0], "run", "--no-restarts",
		"--pinpoint-root", s.opts.PinpointRoot)
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderr
	runErr := cmd.Run()
	if runErr.Error() == "exit status 2" {
		s.logger.Info("exited from run --no-restarts")
		return runErr
	}
	err = errFile.Sync()
	if err != nil {
		return fmt.Errorf("could not sync file for err output: %v", err)
	}
	err = errFile.Close()
	if err != nil {
		return fmt.Errorf("could not close file for err output: %v", err)
	}

	size, err := fileSize(errFileLoc)
	if err != nil {
		return fmt.Errorf("could not check size of file for err output: %v", err)
	}
	if size == 0 {
		// only keep files with actual crashes
		err := os.Remove(errFileLoc)
		if err != nil {
			s.logger.Error("could not remove empty file for err output", "err", err)
		}
	} else {
		// if there was a crash create a metadata file
		data := struct {
			CrashDate time.Time `json:"crash_date"`
		}{}
		data.CrashDate = time.Now()
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		jsonLoc := errFileLoc + ".json"
		err = fs.WriteToTempAndRename(bytes.NewReader(b), jsonLoc)
		if err != nil {
			return fmt.Errorf("could not write crash metadata, err: %v", err)
		}
	}
	return runErr
}

func fileSize(loc string) (int64, error) {
	f, err := os.Open(loc)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}
