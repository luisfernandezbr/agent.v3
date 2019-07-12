package cmdexport

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/pinpt/agent.next/pkg/agentconf"
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

func Run(opts Opts) error {
	exp := newExport(opts)
	defer exp.Destroy()
	return nil
}

type Opts struct {
	Logger       hclog.Logger
	Config       *agentconf.Config
	PinpointRoot string
}

type export struct {
	logger       hclog.Logger
	config       *agentconf.Config
	pluginClient *plugin.Client
	sessions     *sessions

	locs fsconf.Locs

	stderr *bytes.Buffer

	integrations  map[string]*integration
	lastProcessed *jsonstore.Store

	gitProcessingRepos chan repoProcess
}

func newExport(opts Opts) *export {
	if opts.PinpointRoot == "" {
		root, err := fsconf.DefaultRoot()
		if err != nil {
			panic(err)
		}
		opts.PinpointRoot = root
	}

	s := &export{}
	s.logger = opts.Logger
	s.config = opts.Config
	s.locs = fsconf.New(opts.PinpointRoot)

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

	s.setupIntegrations()
	s.runExports()
	close(s.gitProcessingRepos)

	s.logger.Info("waiting for git repo processing to complete")
	<-gitProcessingDone

	return s
}

type repoProcess struct {
	Access     gitclone.AccessDetails
	ID         string
	CustomerID string
}

func (s *export) gitProcessing() error {
	ctx := context.Background()

	for repo := range s.gitProcessingRepos {
		opts := exportrepo.Opts{
			Logger:        s.logger,
			RepoAccess:    repo.Access,
			Sessions:      s.sessions.outsession,
			RepoID:        repo.ID,
			CustomerID:    repo.CustomerID,
			LastProcessed: s.lastProcessed,
		}
		exp := exportrepo.New(opts, s.locs)
		err := exp.Run(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *export) setupIntegrations() {
	integrationNames := s.config.GetEnabledIntegrations()
	s.logger.Info("enabled integrations", "names", integrationNames)

	integrations := make(chan *integration)
	go func() {
		wg := sync.WaitGroup{}
		for _, name := range integrationNames {
			wg.Add(1)
			name := name
			go func() {
				defer wg.Done()
				integration, err := newIntegration(s, name)
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

func (s *export) runExports() {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	errored := map[string]error{}
	erroredMu := sync.Mutex{}

	configPinpoint := rpcdef.ExportConfigPinpoint{
		CustomerID: s.config.Pinpoint.CustomerID,
	}

	for name, integration := range s.integrations {
		wg.Add(1)
		name := name
		integration := integration
		go func() {
			defer wg.Done()
			rerr := func(err error) {
				erroredMu.Lock()
				errored[name] = err
				erroredMu.Unlock()
				s.logger.Error("Export failed", "integration", name, "err", err)
			}

			s.logger.Info("Export starting", "integration", name, "log_file", integration.logFile.Name())

			integrationConfig, err := s.config.IntegrationConfig(name)
			if err != nil {
				rerr(err)
				return
			}
			if len(integrationConfig) == 0 {
				rerr(fmt.Errorf("empty config for integration: %v", name))
				return
			}
			exportConfig := rpcdef.ExportConfig{
				Pinpoint:    configPinpoint,
				Integration: integrationConfig,
			}
			_, err = integration.rpcClient.Export(ctx, exportConfig)
			if err != nil {
				rerr(err)
				return
			}
			s.logger.Info("Export success", "integration", name)
		}()
	}
	wg.Wait()

	for name, integration := range s.integrations {
		if errored[name] != nil {
			s.logger.Error("Export failed", "integration", name, "err", errored[name])
			panicOut, err := integration.CloseAndDetectPanic()
			if panicOut != "" {
				fmt.Println("Panic in integration", name)
				fmt.Println(panicOut)
			}
			if err != nil {
				s.logger.Error("Could not close integration", "err", err)
			}
		} else {
			s.logger.Info("Export succeded", "integration", name)
		}
	}

	if len(errored) > 0 {
		s.logger.Error("Some exports failed", "failed", len(errored), "succeded", len(s.integrations)-len(errored))
	} else {
		s.logger.Info("Exports completed", "succeded", len(s.integrations))
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
