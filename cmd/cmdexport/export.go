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
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/fs"
	"github.com/pinpt/agent/pkg/integrationid"
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
type Integration = cmdintegration.Integration

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

	b, err := json.Marshal(exportResults)
	if err != nil {
		return err
	}

	_, err = opts.Output.Write(b)
	if err != nil {
		return err
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

	gitSessions map[integrationid.ID]expsessions.ID
	// map[integration.ID]map[repoID]error
	gitResults map[integrationid.ID]map[string]error
}

type gitRepoFetch struct {
	integrationID integrationid.ID
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
		err := integration.Close()
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

	err = s.logLastProcessedTimestamps()
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

	err = s.SetupIntegrations(func(in integrationid.ID) rpcdef.Agent {
		return newAgentDelegate(s, s.sessions.expsession, in)
	})
	if err != nil {
		rerr = err
		return
	}

	memorylogs.Start(ctx, s.Logger, 5*time.Second)

	runResult := s.runExports()
	close(s.gitProcessingRepos)

	hadGitErrors := false
	select {
	case hadGitErrors = <-gitProcessingDone:
	case <-time.After(1 * time.Second):
		// only log this if there is actual work needed for git repos
		s.Logger.Info("Waiting for git repo processing to complete")
		hadGitErrors = <-gitProcessingDone
	}

	err = s.updateLastProcessedTimestamps(startTime)
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

	err = s.printExportRes(runResult, hadGitErrors)
	if err != nil {
		rerr = err
		return
	}

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

	return s.formatResults(runResult), nil
}

type Result struct {
	Integrations []ResultIntegration `json:"integrations"`
}

type ResultIntegration struct {
	ID       string          `json:"id"`
	Error    string          `json:"error"`
	Projects []ResultProject `json:"projects"`
}

type ResultProject struct {
	rpcdef.ExportProject
	HasGitRepo bool   `json:"has_git_repo"`
	GitError   string `json:"git_error"`
}

func (s *export) formatResults(runResult map[integrationid.ID]runResult) Result {
	gitResults := s.gitResults

	resAll := Result{}
	for id, res0 := range runResult {
		res := ResultIntegration{}
		res.ID = id.String()
		if res0.Err != nil {
			res.Error = res0.Err.Error()
		}
		for _, project0 := range res0.Res.Projects {
			project := ResultProject{}
			project.ExportProject = project0
			gitErr, ok := gitResults[id][project.ID]
			if ok {
				project.HasGitRepo = true
				if gitErr != nil {
					project.GitError = gitErr.Error()
				}
			}
			res.Projects = append(res.Projects, project)
		}
		resAll.Integrations = append(resAll.Integrations, res)
	}

	return resAll
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

func (s *export) logLastProcessedTimestamps() error {
	lastExport := map[integrationid.ID]string{}
	for _, ino := range s.Opts.Integrations {
		in, err := ino.ID()
		if err != nil {
			return err
		}
		v := s.lastProcessed.Get(in.String())
		if v != nil {
			ts, ok := v.(string)
			if !ok {
				return errors.New("not a valid value saved in last processed key")
			}
			lastExport[in] = ts
		} else {
			lastExport[in] = ""
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

func (s *export) updateLastProcessedTimestamps(startTime time.Time) error {
	for _, ino := range s.Opts.Integrations {
		in, err := ino.ID()
		if err != nil {
			return err
		}
		err = s.lastProcessed.Set(startTime.Format(time.RFC3339), in.String())
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

func (s *export) runExports() map[integrationid.ID]runResult {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	res := map[integrationid.ID]runResult{}
	resMu := sync.Mutex{}

	for _, integration := range s.Integrations {
		wg.Add(1)
		integration := integration
		go func() {
			defer wg.Done()
			start := time.Now()
			id := integration.ID
			ret := func(err error, exportRes rpcdef.ExportResult) {
				resMu.Lock()
				res[id] = runResult{
					Duration: time.Since(start),
					Err:      err,
					Res:      exportRes,
				}
				resMu.Unlock()
				if err != nil {
					s.Logger.Error("Export failed", "integration", id, "dur", time.Since(start).String(), "err", err)
					return
				}
				s.Logger.Info("Export success", "integration", id, "dur", time.Since(start).String())
			}

			s.Logger.Info("Export starting", "integration", id)

			exportConfig, ok := s.ExportConfigs[id]
			if !ok {
				panic("no config for integration")
			}

			exportRes, err := integration.RPCClient().Export(ctx, exportConfig)
			ret(err, exportRes)
		}()
	}
	wg.Wait()

	return res
}

// printExportRes show info on which exports works and which failed.
func (s *export) printExportRes(res map[integrationid.ID]runResult, gitHadErrors bool) error {
	s.Logger.Info("Printing export results for all integrations")

	var successNoGit, failedNoGit []integrationid.ID

	for id, integration := range s.Integrations {
		ires := res[id]
		if ires.Err != nil {
			s.Logger.Error("Export failed", "integration", id, "dur", ires.Duration.String(), "err", ires.Err)
			if err := s.Command.CloseOnlyIntegrationAndHandlePanic(integration); err != nil {
				s.Logger.Error("Could not close integration", "err", err)
			}
			failedNoGit = append(failedNoGit, id)
			continue
		}

		s.Logger.Info("Export success", "integration", id, "dur", ires.Duration.String())
		successNoGit = append(successNoGit, id)
	}

	dur := time.Since(s.StartTime)

	successAll := successNoGit
	failedAll := failedNoGit

	if gitHadErrors {
		failedAll = append(failedAll, integrationid.ID{Name: "git"})
	} else {
		successAll = append(failedAll, integrationid.ID{Name: "git"})
	}

	if len(failedAll) > 0 {
		s.Logger.Error("Some exports failed", "failed", failedAll, "succeeded", successAll, "dur", dur.String())
		// Only mark complete run as failed when integrations fail, git repo errors should not fail those, we only log and retry in incrementals
		if len(failedNoGit) > 0 {
			return errors.New("One or more integrations failed, failing export")
		} else {
			s.Logger.Error("Git processing failed on one or more repos. We are not marking whole export failed in this case. See the logs for details.")
		}
		return nil
	}

	s.Logger.Info("Exports completed", "succeeded", successAll, "dur", dur.String())
	return nil
}
