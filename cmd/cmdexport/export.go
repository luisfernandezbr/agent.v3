package cmdexport

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/mitchellh/go-homedir"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent.next/cmd/cmdexport/process"
	"github.com/pinpt/agent.next/pkg/jsonstore"
	"github.com/pinpt/agent.next/pkg/outsession"
	"github.com/pinpt/agent.next/rpcdef"
)

type Opts struct {
	Logger  hclog.Logger
	WorkDir string
}

type export struct {
	logger       hclog.Logger
	pluginClient *plugin.Client
	sessions     *sessions

	dirs exportDirs

	stderr *bytes.Buffer

	integrations  map[string]*integration
	lastProcessed *jsonstore.Store
}

type exportDirs struct {
	sessions string
	logs     string
}

func newExport(opts Opts) *export {
	if opts.WorkDir == "" {
		dir, err := homedir.Dir()
		if err != nil {
			panic(err)
		}
		opts.WorkDir = filepath.Join(dir, ".pinpoint", "v2", "work")
	}

	s := &export{}
	s.logger = opts.Logger
	s.dirs = exportDirs{
		sessions: filepath.Join(opts.WorkDir, "sessions"),
		logs:     filepath.Join(opts.WorkDir, "logs"),
	}
	lastProcessedFile := filepath.Join(opts.WorkDir, "last_processed.json")
	var err error
	s.lastProcessed, err = jsonstore.New(lastProcessedFile)
	if err != nil {
		panic(err)
	}

	s.sessions = newSessions(s.logger, s, s.dirs.sessions)

	s.setupIntegrations()
	s.runExports()
	return s
}

func (s *export) setupIntegrations() {
	integrations := make(chan *integration)
	go func() {
		wg := sync.WaitGroup{}
		for _, name := range []string{"github"} {
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

	for name, integration := range s.integrations {
		wg.Add(1)
		name := name
		integration := integration
		go func() {
			defer wg.Done()
			s.logger.Info("Export starting", "integration", name, "log_file", integration.logFile.Name())
			err := integration.rpcClient.Export(ctx)
			if err != nil {
				erroredMu.Lock()
				errored[name] = err
				erroredMu.Unlock()
				s.logger.Error("Export failed", "integration", name, "err", err)
			} else {
				s.logger.Info("Export success", "integration", name)
			}
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
