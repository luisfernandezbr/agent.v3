package cmdexport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pinpt/agent.next/pkg/exportrepo"
	"github.com/pinpt/agent.next/pkg/gitclone"
	"github.com/pinpt/agent.next/pkg/integrationid"

	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/pinpt/agent.next/pkg/jsonstore"
	"github.com/pinpt/agent.next/rpcdef"
)

type Opts struct {
	cmdintegration.Opts
	ReprocessHistorical bool
}

type AgentConfig = cmdintegration.AgentConfig
type Integration = cmdintegration.Integration

func Run(opts Opts) error {
	exp, err := newExport(opts)
	if err != nil {
		return err
	}
	defer exp.Destroy()
	return nil
}

type export struct {
	*cmdintegration.Command

	pluginClient *plugin.Client
	sessions     *sessions

	stderr *bytes.Buffer

	lastProcessed *jsonstore.Store

	gitProcessingRepos chan rpcdef.GitRepoFetch
}

func newExport(opts Opts) (*export, error) {
	s := &export{}

	startTime := time.Now()

	var err error
	s.Command, err = cmdintegration.NewCommand(opts.Opts)
	if err != nil {
		return nil, err
	}

	if opts.ReprocessHistorical {
		s.Logger.Info("Starting export. ReprocessHistorical is true, discarding incremental checkpoints")
		err := s.discardIncrementalData()
		if err != nil {
			return nil, err
		}
	} else {
		s.Logger.Info("Starting export. ReprocessHistorical is false, will use incremental checkpoints if available.")
	}

	err = os.MkdirAll(s.Locs.State, 0777)
	if err != nil {
		return nil, fmt.Errorf("could not create dir to save state, err: %v", err)
	}

	s.lastProcessed, err = jsonstore.New(s.Locs.LastProcessedFile)
	if err != nil {
		return nil, err
	}

	err = s.logLastProcessedTimestamps()
	if err != nil {
		return nil, err
	}

	s.sessions, err = newSessions(s.Logger, s, opts.ReprocessHistorical)
	if err != nil {
		return nil, err
	}

	s.gitProcessingRepos = make(chan rpcdef.GitRepoFetch, 100000)

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
		return nil, err
	}

	exportRes := s.runExports()
	close(s.gitProcessingRepos)

	hadErrors := false
	select {
	case hadErrors = <-gitProcessingDone:
	case <-time.After(1 * time.Second):
		// only log this is there is actual work needed for git repos
		s.Logger.Info("Waiting for git repo processing to complete")
		hadErrors = <-gitProcessingDone
	}

	err = s.updateLastProcessedTimestamps(startTime)
	if err != nil {
		return nil, err
	}

	err = s.lastProcessed.Save()
	if err != nil {
		s.Logger.Error("could not save updated last_processed file", "err", err)
		return nil, err
	}

	err = s.sessions.Close()
	if err != nil {
		s.Logger.Error("could not close sessions", "err", err)
		return nil, err
	}

	s.printExportRes(exportRes, hadErrors)

	return s, nil
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

	s.Logger.Info("Last processed timestamps", "v", lastExport)
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

func (s *export) gitProcessing() (hadErrors bool, _ error) {
	logger := s.Logger.Named("git")

	if s.Opts.AgentConfig.SkipGit {
		logger.Warn("SkipGit is true, skipping git clone and ripsrc for all repos")
		for range s.gitProcessingRepos {
		}
		return false, nil
	}

	logger.Info("starting git repo processing")

	i := 0
	reposFailedRevParse := 0
	var start time.Time

	ctx := context.Background()
	sessionRoot, _, err := s.sessions.expsession.SessionRootTracking(integrationid.ID{
		Name: "git",
	}, "git")
	if err != nil {
		logger.Error("could not create session for git export", "err", err.Error())
		return true, nil
	}

	resErrors := map[string]error{}
	var ripsrcDuration time.Duration
	var gitClonecDuration time.Duration
	for fetch := range s.gitProcessingRepos {
		if i == 0 {
			start = time.Now()
		}
		i++
		access := gitclone.AccessDetails{}
		access.URL = fetch.URL

		opts := exportrepo.Opts{
			Logger:     s.Logger.With("c", i),
			CustomerID: s.Opts.AgentConfig.CustomerID,
			RepoID:     fetch.RepoID,
			UniqueName: fetch.UniqueName,
			RefType:    fetch.RefType,

			LastProcessed: s.lastProcessed,
			RepoAccess:    access,

			CommitURLTemplate: fetch.CommitURLTemplate,
			BranchURLTemplate: fetch.BranchURLTemplate,

			Sessions:      s.sessions.expsession,
			SessionRootID: sessionRoot,
		}
		exp := exportrepo.New(opts, s.Locs)
		repoDirName, duration, err := exp.Run(ctx)

		s.sessions.expsession.Progress(sessionRoot, i, 0)

		if err == exportrepo.ErrRevParseFailed {
			reposFailedRevParse++
			continue
		}
		if err != nil {
			logger.Error("Error processing git repo", "repo", repoDirName, "err", err)
			resErrors[repoDirName] = err
		} else {
			logger.Info("Finished processing git repo", "repo", repoDirName)
		}
		ripsrcDuration += duration.Ripsrc
		gitClonecDuration += duration.Clone
	}

	if i == 0 {
		logger.Info("Finished git repo processing: No git repos found")
		return false, nil
	}

	if reposFailedRevParse != 0 {
		logger.Warn("Skipped ripsrc on empty repos", "repos", reposFailedRevParse)
	}

	if len(resErrors) != 0 {
		for k, err := range resErrors {
			logger.Error("Error processing git repo", "repo", k, "err", err)
		}
		logger.Error("Error in git repo processing", "count", i, "dur", time.Since(start).String(), "repos_failed", len(resErrors))
		return true, nil

	}

	err = s.sessions.expsession.Done(sessionRoot, nil)
	if err != nil {
		logger.Error("could not close session for git export", "err", err.Error())
		return true, nil
	}

	logger.Info("Finished git repo processing", "count", i,
		"duration", time.Since(start).String(),
		"gitclone", gitClonecDuration.String(),
		"ripsrc", ripsrcDuration.String(),
	)

	return false, nil
}

type runResult struct {
	Err      error
	Duration time.Duration
}

func (s *export) runExports() map[string]runResult {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	res := map[string]runResult{}
	resMu := sync.Mutex{}

	for name, integration := range s.Integrations {
		wg.Add(1)
		name := name
		integration := integration
		go func() {
			defer wg.Done()
			start := time.Now()

			ret := func(err error) {
				resMu.Lock()
				res[name] = runResult{Err: err, Duration: time.Since(start)}
				resMu.Unlock()
				if err != nil {
					s.Logger.Error("Export failed", "integration", name, "dur", time.Since(start).String(), "err", err)
					return
				}
				s.Logger.Info("Export success", "integration", name, "dur", time.Since(start).String())
			}

			s.Logger.Info("Export starting", "integration", name)

			exportConfig, ok := s.ExportConfigs[name]
			if !ok {
				panic("no config for integration")
			}
			_, err := integration.RPCClient().Export(ctx, exportConfig)
			if err != nil {
				ret(err)
				return
			}

			ret(nil)
		}()
	}
	wg.Wait()

	return res
}

func (s *export) printExportRes(res map[string]runResult, gitHadErrors bool) {
	s.Logger.Debug("Printing export results for all integrations")

	var successNames []string
	var failedNames []string

	for name, integration := range s.Integrations {
		ires := res[name]
		if ires.Err != nil {
			s.Logger.Error("Export failed", "integration", name, "dur", ires.Duration.String(), "err", ires.Err)
			panicOut, err := integration.CloseAndDetectPanic()
			if panicOut != "" {
				// This is already printed in integration. But we will repeat output at the end, so it's not lost.
				fmt.Println("Panic in integration", name)
				fmt.Println(panicOut)
			}
			if err != nil {
				s.Logger.Error("Could not close integration", "err", err)
			}
			failedNames = append(failedNames, name)
			continue
		}

		s.Logger.Info("Export success", "integration", name, "dur", ires.Duration.String())
		successNames = append(successNames, name)
	}

	dur := time.Since(s.StartTime)

	if gitHadErrors {
		failedNames = append(failedNames, "git")
	}

	if len(failedNames) > 0 {
		s.Logger.Error("Some exports failed", "failed", failedNames, "succeded", successNames, "dur", dur.String())
	} else {
		s.Logger.Info("Exports completed", "succeded", successNames, "dur", dur.String())
	}
}

func (s *export) Destroy() {
	for _, integration := range s.Integrations {
		err := integration.Close()
		if err != nil {
			panic(err)
		}
	}
}
