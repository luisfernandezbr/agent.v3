package cmdexport

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/pinpt/agent.next/pkg/commitusers"
	"github.com/pinpt/agent.next/pkg/exportrepo"
	"github.com/pinpt/agent.next/pkg/gitclone"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent.next/cmd/cmdexport/process"
	"github.com/pinpt/agent.next/cmd/cmdintegration"
	"github.com/pinpt/agent.next/pkg/jsonstore"
	"github.com/pinpt/agent.next/pkg/outsession"
	"github.com/pinpt/agent.next/rpcdef"
)

type Opts struct {
	cmdintegration.Opts
	ReprocessHistorical bool `json:"reprocess_historical"`
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

	gitProcessingRepos chan repoProcess
}

func newExport(opts Opts) (*export, error) {
	s := &export{}
	s.Command = cmdintegration.NewCommand(opts.Opts)

	if opts.ReprocessHistorical {
		s.Logger.Info("ReprocessHistorical is true, discarding incrmental checkpoints")
		err := s.discardIncrementalData()
		if err != nil {
			return nil, err
		}
	} else {
		s.Logger.Info("ReprocessHistorical is false, will use incremental checkpoints if available")
	}

	var err error
	s.lastProcessed, err = jsonstore.New(s.Locs.LastProcessedFile)
	if err != nil {
		return nil, err
	}

	s.sessions = newSessions(s.Logger, s, s.Locs.Uploads)

	s.gitProcessingRepos = make(chan repoProcess, 100000)

	gitProcessingDone := make(chan bool)

	go func() {
		hadErrors, err := s.gitProcessing()
		if err != nil {
			panic(err)
		}
		gitProcessingDone <- hadErrors
	}()

	s.SetupIntegrations(agentDelegate{export: s})

	exportRes := s.runExports()
	close(s.gitProcessingRepos)

	s.Logger.Info("Waiting for git repo processing to complete")
	hadErrors := <-gitProcessingDone

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

type repoProcess struct {
	Access            gitclone.AccessDetails
	ID                string
	CommitURLTemplate string
}

func (s *export) gitProcessing() (hadErrors bool, _ error) {
	if s.Opts.AgentConfig.SkipGit {
		s.Logger.Warn("SkipGit is true, skipping git clone and ripsrc for all repos")
		for range s.gitProcessingRepos {

		}
		return false, nil
	}

	i := 0
	reposFailedRevParse := 0
	var start time.Time

	ctx := context.Background()

	resErrors := map[string]error{}

	for repo := range s.gitProcessingRepos {
		if i == 0 {
			start = time.Now()
		}
		i++
		opts := exportrepo.Opts{
			Logger:     s.Logger.With("c", i),
			CustomerID: s.Opts.AgentConfig.CustomerID,
			RepoID:     repo.ID,

			Sessions: s.sessions.outsession,

			LastProcessed: s.lastProcessed,
			RepoAccess:    repo.Access,

			CommitURLTemplate: repo.CommitURLTemplate,
		}
		exp := exportrepo.New(opts, s.Locs)
		repoDirName, err := exp.Run(ctx)
		if err == exportrepo.ErrRevParseFailed {
			reposFailedRevParse++
			continue
		}
		if err != nil {
			s.Logger.Error("Error processing git repo", "repo", repoDirName, "err", err)
			resErrors[repoDirName] = err
		}
	}

	if i == 0 {
		s.Logger.Warn("Finished git repo processing: No git repos found")
		return false, nil
	}

	if reposFailedRevParse != 0 {
		s.Logger.Warn("Skipped ripsrc on empty repos", "repos", reposFailedRevParse)
	}

	if len(resErrors) != 0 {
		for k, err := range resErrors {
			s.Logger.Error("Error processing git repo", "repo", k, "err", err)
		}
		s.Logger.Error("Error in git repo processing", "count", i, "dur", time.Since(start), "repos_failed", len(resErrors))
		return true, nil

	}

	s.Logger.Info("Finished git repo processing", "count", i, "dur", time.Since(start))
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

	configPinpoint := rpcdef.ExportConfigPinpoint{
		CustomerID: s.Opts.AgentConfig.CustomerID,
	}

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
					s.Logger.Error("Export failed", "integration", name, "dur", time.Since(start), "err", err)
					return
				}
				s.Logger.Info("Export success", "integration", name, "dur", time.Since(start))
			}

			s.Logger.Info("Export starting", "integration", name, "log_file", integration.LogFile())

			integrationDef, ok := s.IntegrationConfigs[name]
			if !ok {
				panic("no config for integration")
			}
			integrationConfig := integrationDef.Config
			if len(integrationConfig) == 0 {
				ret(fmt.Errorf("empty config for integration: %v", name))
				return
			}
			exportConfig := rpcdef.ExportConfig{
				Pinpoint:    configPinpoint,
				Integration: integrationConfig,
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

	s.printExportRes(res, false)

	return res
}

func (s *export) printExportRes(res map[string]runResult, gitHadErrors bool) {
	var successNames []string
	var failedNames []string

	for name, integration := range s.Integrations {
		ires := res[name]
		if ires.Err != nil {
			s.Logger.Error("Export failed", "integration", name, "dur", ires.Duration, "err", ires.Err)
			panicOut, err := integration.CloseAndDetectPanic()
			if panicOut != "" {
				fmt.Println("Panic in integration", name)
				fmt.Println(panicOut)
			}
			if err != nil {
				s.Logger.Error("Could not close integration", "err", err)
			}
			failedNames = append(failedNames, name)
			continue
		}

		s.Logger.Info("Export success", "integration", name, "dur", ires.Duration)
		successNames = append(successNames, name)
	}

	dur := time.Since(s.StartTime)

	if gitHadErrors {
		failedNames = append(failedNames, "git")
	}

	if len(failedNames) > 0 {
		s.Logger.Error("Some exports failed", "failed", failedNames, "succeded", successNames, "dur", dur)
	} else {
		s.Logger.Info("Exports completed", "succeded", successNames, "dur", dur)
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

type sessions struct {
	logger      hclog.Logger
	export      *export
	outsession  *outsession.Manager
	commitUsers *process.CommitUsers
}

func newSessions(logger hclog.Logger, export *export, outputDir string) *sessions {

	s := &sessions{}
	s.logger = logger
	s.export = export
	s.commitUsers = process.NewCommitUsers()

	s.outsession = outsession.New(outsession.Opts{
		Logger:        logger,
		OutputDir:     outputDir,
		LastProcessed: export.lastProcessed,
	})
	return s
}

func (s *sessions) new(modelType string) (
	sessionID string, lastProcessed interface{}, _ error) {

	id, lastProcessed, err := s.outsession.NewSession(modelType)
	if err != nil {
		return "", nil, err
	}
	return idToString(id), lastProcessed, nil
}

func (s *sessions) ExportDone(sessionID string, lastProcessed interface{}) error {
	id := idFromString(sessionID)
	return s.outsession.Done(id, lastProcessed)
}

func idToString(id outsession.ID) string {
	return strconv.Itoa(int(id))
}

func idFromString(str string) outsession.ID {
	id, err := strconv.Atoi(str)
	if err != nil {
		panic(err)
	}
	return outsession.ID(id)
}

func (s *sessions) Write(sessionID string, objs []rpcdef.ExportObj) error {
	if len(objs) == 0 {
		s.logger.Warn("no objects passed to write")
		return nil
	}

	id := idFromString(sessionID)
	modelType := s.outsession.GetModelType(id)
	//s.logger.Info("writing objects", "type", modelType, "count", len(objs))

	if modelType == commitusers.TableName {
		var res []rpcdef.ExportObj
		for _, obj := range objs {
			obj2, err := s.commitUsers.Transform(obj.Data.(map[string]interface{}))
			if err != nil {
				return err
			}
			if obj2 != nil {
				res = append(res, rpcdef.ExportObj{Data: obj2})
			}
		}
		if len(res) == 0 {
			// no new users
			return nil
		}
		objs = res
	}

	var data []map[string]interface{}
	for _, obj := range objs {
		data = append(data, obj.Data.(map[string]interface{}))
	}
	return s.outsession.Write(id, data)
}
