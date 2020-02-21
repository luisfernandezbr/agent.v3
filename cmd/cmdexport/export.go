package cmdexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pinpt/agent/pkg/deviceinfo"
	"github.com/pinpt/agent/pkg/expin"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/fs"
	"github.com/pinpt/agent/pkg/memorylogs"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent/cmd/cmdintegration"
	"github.com/pinpt/agent/pkg/jsonstore"
	"github.com/pinpt/agent/rpcdef"
)

type Opts struct {
	cmdintegration.Opts
	Output              io.Writer
	ReprocessHistorical bool
}

type AgentConfig = cmdintegration.AgentConfig

func Run(opts Opts) error {
	exp, err := newExport(opts)
	if err != nil {
		return err
	}

	exportResults, err := exp.Run()
	if err != nil {
		return err
	}

	err = exp.Destroy()
	if err != nil {
		return err
	}

	exportResults.Log(opts.Logger)

	if opts.Output != nil {
		b, err := json.Marshal(exportResults)
		if err != nil {
			return err
		}

		_, err = opts.Output.Write(b)
		if err != nil {
			return err
		}
	}

	return nil
}

type export struct {
	*cmdintegration.Command

	pluginClient *plugin.Client
	sessions     *sessions

	stderr *bytes.Buffer

	lastProcessed *jsonstore.Store

	gitProcessingRepos chan gitRepoFetch
	deviceInfo         deviceinfo.CommonInfo

	opts Opts

	gitSessions map[expin.Export]expsessions.ID
	// map[integration.ID]map[repoID]error
	gitResults map[expin.Export]map[string]error

	isIncremental map[expin.Export]bool
}

type gitRepoFetch struct {
	exp expin.Export
	rpcdef.GitRepoFetch
}

func newExport(opts Opts) (*export, error) {
	s := &export{}
	s.opts = opts

	var err error
	s.Command, err = cmdintegration.NewCommand(opts.Opts)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *export) Destroy() error {
	for _, integration := range s.Integrations {
		err := integration.ILoader.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *export) Run() (_ Result, rerr error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := s.opts

	startTime := time.Now()

	s.deviceInfo = deviceinfo.CommonInfo{
		CustomerID: opts.AgentConfig.CustomerID,
		DeviceID:   s.EnrollConf.DeviceID,
		SystemID:   s.EnrollConf.SystemID,
		Root:       opts.Opts.AgentConfig.PinpointRoot,
	}

	s.Command.Deviceinfo = s.deviceInfo

	if opts.ReprocessHistorical {
		s.Logger.Info("Starting export. ReprocessHistorical is true, discarding incremental checkpoints")
		err := s.discardIncrementalData()
		if err != nil {
			rerr = err
			return
		}
	} else {
		s.Logger.Info("Starting export. ReprocessHistorical is false, will use incremental checkpoints if available.")
	}

	var err error
	s.lastProcessed, err = jsonstore.New(s.Locs.LastProcessedFile)
	if err != nil {
		rerr = err
		return
	}

	err = s.checkIfIncremental()
	if err != nil {
		rerr = err
		return
	}

	trackProgress := os.Getenv("PP_AGENT_NO_TRACK_PROGRESS") == ""
	s.sessions, err = newSessions(s.Logger, s, opts.ReprocessHistorical, trackProgress)
	if err != nil {
		rerr = err
		return
	}

	s.gitProcessingRepos = make(chan gitRepoFetch, 100000)

	gitProcessingDone := make(chan bool)

	go func() {
		hadErrors, err := s.gitProcessing()
		if err != nil {
			panic(err)
		}
		gitProcessingDone <- hadErrors
	}()

	err = s.SetupIntegrations(func(exp expin.Export) rpcdef.Agent {
		return newAgentDelegate(s, s.sessions.expsession, exp)
	})
	if err != nil {
		rerr = err
		return
	}

	memorylogs.Start(ctx, s.Logger, 5*time.Second)

	runResult := s.runExports()
	close(s.gitProcessingRepos)

	select {
	case <-gitProcessingDone:
	case <-time.After(1 * time.Second):
		// only log this if there is actual work needed for git repos
		s.Logger.Info("Waiting for git repo processing to complete")
		<-gitProcessingDone
	}

	err = s.updateLastProcessedTimestampsForIncrementalCheck(startTime)
	if err != nil {
		rerr = err
		return
	}

	err = s.lastProcessed.Save()
	if err != nil {
		s.Logger.Error("could not save updated last_processed file", "err", err)
		rerr = err
		return
	}

	err = s.sessions.Close()
	if err != nil {
		s.Logger.Error("could not close sessions", "err", err)
		rerr = err
		return
	}

	s.handleIntegrationPanics(runResult)

	tempFiles, err := s.tempFilesInUploads()
	if err != nil {
		s.Logger.Error("could not check uploads dir for errors", "err", err)
		rerr = err
		return
	}
	if len(tempFiles) != 0 {
		rerr = fmt.Errorf("found temp sessions files in upload dir, files: %v", tempFiles)
		return

	}

	s.Logger.Info("No temp files found in upload dir, all sessions closed successfully.")

	return s.formatResults(runResult, startTime), nil
}

func (s *export) tempFilesInUploads() ([]string, error) {
	uploadsExist, err := fs.Exists(s.Locs.Uploads)
	if err != nil {
		return nil, fmt.Errorf("Could not check if uploads dir exist: %v", err)
	}
	if !uploadsExist {
		return nil, nil
	}

	var rec func(string) ([]string, error)
	rec = func(p string) (res []string, rerr error) {
		items, err := ioutil.ReadDir(p)
		if err != nil {
			rerr = err
			return
		}
		for _, item := range items {
			n := filepath.Join(p, item.Name())
			if item.IsDir() {
				sr, err := rec(n)
				if err != nil {
					rerr = err
					return
				}
				res = append(res, sr...)
				continue
			}
			if !strings.HasSuffix(n, ".temp.gz") {
				continue
			}
			res = append(res, n)
		}
		return
	}
	return rec(s.Locs.Uploads)
}

func (s *export) discardIncrementalData() error {
	err := os.RemoveAll(s.Locs.LastProcessedFile)
	if err != nil {
		return err
	}
	return os.RemoveAll(s.Locs.RipsrcCheckpoints)
}

func (s *export) checkIfIncremental() error {
	lastExport := map[expin.Export]string{}
	s.isIncremental = map[expin.Export]bool{}
	for exp, in := range s.Integrations {
		v := s.lastProcessed.Get(in.Export.String())
		if v != nil {
			ts, ok := v.(string)
			if !ok {
				return errors.New("not a valid value saved in last processed key")
			}
			lastExport[exp] = ts
			s.isIncremental[exp] = true
		} else {
			lastExport[exp] = ""
			s.isIncremental[exp] = false
		}
	}

	// convert for log
	obj := map[string]string{}
	for k, v := range lastExport {
		obj[k.String()] = v
	}
	s.Logger.Info("Last processed timestamps", "v", obj)
	return nil
}

func (s *export) updateLastProcessedTimestampsForIncrementalCheck(startTime time.Time) error {
	for exp := range s.Integrations {
		err := s.lastProcessed.Set(startTime.Format(time.RFC3339), exp.String())
		if err != nil {
			return err
		}
	}
	return nil
}

type runResult struct {
	Err      error
	Duration time.Duration
	Res      rpcdef.ExportResult
}

func (s *export) runExports() map[expin.Export]runResult {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	res := map[expin.Export]runResult{}
	resMu := sync.Mutex{}

	for exp, integration := range s.Integrations {
		wg.Add(1)
		exp := exp
		integration := integration
		go func() {
			defer wg.Done()
			start := time.Now()
			ret := func(err error, exportRes rpcdef.ExportResult) {
				resMu.Lock()
				res[exp] = runResult{
					Duration: time.Since(start),
					Err:      err,
					Res:      exportRes,
				}
				resMu.Unlock()
				if err != nil {
					s.Logger.Error("Export failed", "integration", exp.String(), "dur", time.Since(start).String(), "err", err)
					return
				}
				s.Logger.Info("Export success", "integration", exp.String(), "dur", time.Since(start).String())
			}

			s.Logger.Info("Export starting", "integration", exp.String())

			exportRes, err := integration.ILoader.RPCClient().Export(ctx, integration.ExportConfig)
			ret(err, exportRes)
		}()
	}
	wg.Wait()

	return res
}
