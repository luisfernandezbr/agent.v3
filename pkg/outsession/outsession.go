package outsession

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/io"
)

type Manager struct {
	opts   Opts
	logger hclog.Logger

	sessions   map[ID]*session
	sessionsMu sync.RWMutex

	lastID ID
}

type LastProcessedStore interface {
	Get(key ...string) interface{}
	Set(value interface{}, key ...string) error
}

type Opts struct {
	Logger        hclog.Logger
	OutputDir     string
	LastProcessed LastProcessedStore
}

type ID int

func New(opts Opts) *Manager {
	if opts.OutputDir == "" {
		panic("provide OutputDir")
	}
	s := &Manager{}
	s.opts = opts
	s.logger = opts.Logger.Named("outsession")
	s.sessions = map[ID]*session{}
	return s
}

func (s *Manager) NewSession(modelType string) (_ ID, lastProcessed interface{}, _ error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.lastID++
	id := s.lastID
	sess, err := newSession(s.logger, modelType, s.opts.OutputDir, id)
	if err != nil {
		return 0, nil, err
	}
	s.sessions[id] = sess
	if s.opts.LastProcessed != nil {
		lastProcessed = s.opts.LastProcessed.Get(modelType)
	}
	s.logger.Info("create session", "type", modelType, "last_processed_old", lastProcessed)
	return id, lastProcessed, nil
}

func (s *Manager) Write(id ID, objs []map[string]interface{}) error {
	return s.getLocked(id).Write(objs)
}

func (s *Manager) get(id ID) *session {
	return s.sessions[id]
}

func (s *Manager) getLocked(id ID) *session {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	return s.sessions[id]
}

func (s *Manager) GetModelType(id ID) string {
	return s.getLocked(id).modelType
}

func (s *Manager) Done(id ID, lastProcessed interface{}) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	sess := s.get(id)
	if sess == nil {
		panic("could not find session by id")
	}
	modelType := sess.modelType

	err := sess.Close()
	delete(s.sessions, id)
	if err != nil {
		return err
	}
	if s.opts.LastProcessed != nil {
		err = s.opts.LastProcessed.Set(lastProcessed, modelType)
		if err != nil {
			return err
		}
	}
	s.logger.Info("session done", "type", modelType, " last_processed_new", lastProcessed)
	return nil
}

type session struct {
	id        ID
	logger    hclog.Logger
	modelType string
	outputDir string

	streamCreated sync.Once

	stream   *io.JSONStream
	streamMu sync.Mutex
}

func newSession(logger hclog.Logger, modelType string, outputDir string, id ID) (*session, error) {
	s := &session{}
	s.id = id
	s.logger = logger
	s.modelType = modelType
	s.outputDir = outputDir
	return s, nil
}

// Close closes session. Should not be called concurrently with other methods.
func (s *session) Close() error {
	if s.stream == nil {
		// there was no stream, since no objects were sent
		return nil
	}
	return s.stream.Close()
}

func (s *session) createStreamIfNeeded() (rerr error) {
	s.streamCreated.Do(func() {
		err := s.createStream(s.outputDir)
		if err != nil {
			rerr = err
		}
	})
	return
}

func (s *session) createStream(outputDir string) error {
	base := strconv.FormatInt(time.Now().Unix(), 10) + "_" + strconv.Itoa(int(s.id)) + ".json.gz"
	fn := filepath.Join(outputDir, s.modelType, base)
	err := os.MkdirAll(filepath.Dir(fn), 0777)
	if err != nil {
		return err
	}
	stream, err := io.NewJSONStream(fn)
	if err != nil {
		return err
	}
	s.stream = stream
	return nil
}

func (s *session) Write(objs []map[string]interface{}) error {
	err := s.createStreamIfNeeded()
	if err != nil {
		return err
	}

	//s.logger.Debug("writing", "n", len(objs), "model_type", s.modelType)
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	for _, obj := range objs {
		err := s.stream.Write(obj)
		if err != nil {
			return err
		}
	}
	return nil
}
