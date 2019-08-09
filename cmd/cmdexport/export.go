package cmdexport

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pinpt/agent.next/pkg/exportrepo"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/gitclone"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent.next/cmd/cmdexport/process"
	"github.com/pinpt/agent.next/pkg/jsonstore"
	"github.com/pinpt/agent.next/pkg/outsession"
	"github.com/pinpt/agent.next/rpcdef"
)

type Opts struct {
	Logger       hclog.Logger
	AgentConfig  AgentConfig
	Integrations []Integration
}

type AgentConfig struct {
	CustomerID   string `json:"customer_id"`
	PinpointRoot string `json:"pinpoint_root"`
	// SkipGit is a flag for skipping git repo cloning, ripsrc processing, useful when developing
	SkipGit bool `json:"skip_git"`
	// IntegrationsDir is a custom location of the integrations binaries
	IntegrationsDir string `json:"integrations_dir"`
}

type Integration struct {
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config"`
}

func Run(opts Opts) error {
	exp := newExport(opts)
	defer exp.Destroy()
	return nil
}

type export struct {
	opts Opts

	logger       hclog.Logger
	pluginClient *plugin.Client
	sessions     *sessions

	locs fsconf.Locs

	stderr *bytes.Buffer

	integrations  map[string]*integration
	lastProcessed *jsonstore.Store

	gitProcessingRepos chan repoProcess

	integrationConfigs map[string]Integration

	startTime time.Time
}

func newExport(opts Opts) *export {
	s := &export{}
	s.opts = opts
	s.logger = opts.Logger

	s.startTime = time.Now()

	root := opts.AgentConfig.PinpointRoot
	if root == "" {
		v, err := fsconf.DefaultRoot()
		if err != nil {
			panic(err)
		}
		root = v
	}

	s.locs = fsconf.New(root)
	if opts.AgentConfig.IntegrationsDir != "" {
		s.locs.Integrations = opts.AgentConfig.IntegrationsDir
	}

	var err error
	s.lastProcessed, err = jsonstore.New(s.locs.LastProcessedFile)
	if err != nil {
		panic(err)
	}

	s.sessions = newSessions(s.logger, s, s.locs.Uploads)

	s.gitProcessingRepos = make(chan repoProcess, 100000)
	gitProcessingDone := make(chan bool)

	go func() {
		defer func() {
			gitProcessingDone <- true
		}()
		err = s.gitProcessing()
		if err != nil {
			panic(err)
		}
	}()

	s.integrationConfigs = map[string]Integration{}
	for _, in := range opts.Integrations {
		s.integrationConfigs[in.Name] = in
	}

	s.setupIntegrations()
	exportRes := s.runExports()
	close(s.gitProcessingRepos)

	s.logger.Info("Waiting for git repo processing to complete")
	<-gitProcessingDone

	s.printExportRes(exportRes)

	return s
}

type repoProcess struct {
	Access gitclone.AccessDetails
	ID     string
}

func (s *export) gitProcessing() error {
	if s.opts.AgentConfig.SkipGit {
		s.logger.Warn("SkipGit is true, skipping git clone and ripsrc for all repos")
		for range s.gitProcessingRepos {

		}
		return nil
	}

	i := 0
	reposFailedRevParse := 0
	var start time.Time

	ctx := context.Background()
	for repo := range s.gitProcessingRepos {
		if i == 0 {
			start = time.Now()
		}
		i++
		opts := exportrepo.Opts{
			Logger:     s.logger.With("c", i),
			CustomerID: s.opts.AgentConfig.CustomerID,
			RepoID:     repo.ID,

			Sessions: s.sessions.outsession,

			LastProcessed: s.lastProcessed,
			RepoAccess:    repo.Access,
		}
		exp := exportrepo.New(opts, s.locs)
		err := exp.Run(ctx)
		if err == exportrepo.ErrRevParseFailed {
			reposFailedRevParse++
			continue
		}
		if err != nil {
			return err
		}
	}

	if i == 0 {
		s.logger.Warn("Finished git repo processing: No git repos found")
		return nil
	}

	if reposFailedRevParse != 0 {
		s.logger.Warn("Skipped ripsrc on empty repos", "repos", reposFailedRevParse)
	}

	if i != 0 {
		s.logger.Info("Finished git repo processing", "count", i, "dur", time.Since(start))
	}

	return nil
}

func (s *export) setupIntegrations() {
	var integrationNames []string
	for name := range s.integrationConfigs {
		integrationNames = append(integrationNames, name)
	}

	s.logger.Info("running integrations", "integrations", integrationNames)

	integrations := make(chan *integration)
	go func() {
		wg := sync.WaitGroup{}
		for _, name := range integrationNames {
			wg.Add(1)
			name := name
			go func() {
				defer wg.Done()
				integration, err := newIntegration(s, name, s.locs)
				if err != nil {
					panic(err)
				}
				integrations <- integration
			}()
		}
		wg.Wait()
		close(integrations)
	}()
	s.integrations = map[string]*integration{}
	for integration := range integrations {
		s.integrations[integration.name] = integration
	}
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
		CustomerID: s.opts.AgentConfig.CustomerID,
	}

	for name, integration := range s.integrations {
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
					s.logger.Error("Export failed", "integration", name, "dur", time.Since(start), "err", err)
					return
				}
				s.logger.Info("Export success", "integration", name, "dur", time.Since(start))
			}

			s.logger.Info("Export starting", "integration", name, "log_file", integration.logFile.Name())

			integrationDef, ok := s.integrationConfigs[name]
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
			_, err := integration.rpcClient.Export(ctx, exportConfig)
			if err != nil {
				ret(err)
				return
			}

			ret(nil)
		}()
	}
	wg.Wait()

	s.printExportRes(res)

	return res
}

func (s *export) printExportRes(res map[string]runResult) {
	var successNames []string
	var failedNames []string

	for name, integration := range s.integrations {
		ires := res[name]
		if ires.Err != nil {
			s.logger.Error("Export failed", "integration", name, "dur", ires.Duration, "err", ires.Err)
			panicOut, err := integration.CloseAndDetectPanic()
			if panicOut != "" {
				fmt.Println("Panic in integration", name)
				fmt.Println(panicOut)
			}
			if err != nil {
				s.logger.Error("Could not close integration", "err", err)
			}
			failedNames = append(failedNames, name)
			continue
		}

		s.logger.Info("Export success", "integration", name, "dur", ires.Duration)
		successNames = append(successNames, name)
	}

	dur := time.Since(s.startTime)

	if len(failedNames) > 0 {
		s.logger.Error("Some exports failed", "failed", failedNames, "succeded", successNames, "dur", dur)
	} else {
		s.logger.Info("Exports completed", "succeded", successNames, "dur", dur)
	}
}

func (s *export) Destroy() {
	for _, integration := range s.integrations {
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
	s.logger.Info("writing objects", "type", modelType, "count", len(objs))

	if modelType == "sourcecode.commit_user" {
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
