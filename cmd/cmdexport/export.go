package cmdexport

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/mitchellh/go-homedir"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent.next/pkg/jsonstore"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-common/io"
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
	s.sessions = newSessions(s.logger, s, s.dirs.sessions)
	lastProcessedFile := filepath.Join(opts.WorkDir, "last_processed.json")
	var err error
	s.lastProcessed, err = jsonstore.New(lastProcessedFile)
	if err != nil {
		panic(err)
	}

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
	export    *export
	m         map[int]session
	streamDir string
	lastID    int
	logger    hclog.Logger
}

func newSessions(logger hclog.Logger, export *export, streamDir string) *sessions {
	s := &sessions{}
	s.logger = logger
	s.export = export
	s.m = map[int]session{}
	s.streamDir = streamDir
	return s
}

func (s *sessions) new(modelType string) (
	sessionID string, lastProcessed interface{}, _ error) {

	s.lastID++
	id := s.lastID

	base := strconv.FormatInt(time.Now().Unix(), 10) + "_" + strconv.Itoa(id) + ".json.gz"
	fn := filepath.Join(s.streamDir, modelType, base)
	err := os.MkdirAll(filepath.Dir(fn), 0777)
	if err != nil {
		return "", "", err
	}
	stream, err := io.NewJSONStream(fn)
	if err != nil {
		return "", "", err
	}

	s.m[id] = session{
		state:     "started",
		modelType: modelType,
		stream:    stream,
	}

	sessionID = strconv.Itoa(id)
	lastProcessed = s.export.lastProcessed.Get(modelType)

	s.logger.Info("export: session: started", "type", modelType, "last_processed", lastProcessed)

	return sessionID, lastProcessed, nil
}

func (s *sessions) get(sessionID string) session {
	id, err := strconv.Atoi(sessionID)
	if err != nil {
		panic(err)
	}
	return s.m[id]
}

func (s *sessions) ExportDone(sessionID string, lastProcessed interface{}) error {
	sess := s.get(sessionID)

	s.logger.Info("export: session: finished", "type", sess.modelType, "last_processed", lastProcessed)

	err := s.Close(sessionID)
	if err != nil {
		return err
	}

	return s.export.lastProcessed.Set(lastProcessed, sess.modelType)
}

func (s *sessions) Close(sessionID string) error {
	sess := s.get(sessionID)
	err := sess.stream.Close()
	if err != nil {
		return err
	}
	idi, err := strconv.Atoi(sessionID)
	if err != nil {
		return err
	}
	delete(s.m, idi)
	return nil
}

func (s *sessions) Write(sessionID string, objs []rpcdef.ExportObj) error {
	sess := s.get(sessionID)
	s.logger.Info("writing objects", "type", sess.modelType, "count", len(objs))
	for _, obj := range objs {
		err := sess.stream.Write(obj.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

type session struct {
	state     string
	modelType string
	stream    *io.JSONStream
}
