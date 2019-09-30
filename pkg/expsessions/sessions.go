package expsessions

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
)

// Opts are options for New call
type Opts struct {
	Logger        hclog.Logger
	LastProcessed LastProcessedStore

	NewWriter NewWriterFunc

	SendProgress     SendProgressFunc
	SendProgressDone SendProgressDoneFunc
}

type SendProgressFunc func(pp ProgressPath, current, total int)
type SendProgressDoneFunc func(pp ProgressPath)
type NewWriterFunc func(modelName string, id ID) Writer

// ID is the session id
type ID int

// New creates a new manager
func New(opts Opts) *Manager {
	s := &Manager{}
	s.opts = opts
	s.logger = opts.Logger.Named("export-sessions")
	s.sessions = map[ID]*session{}
	return s
}

// LastProcessedStore is the interface for storing last processed information
type LastProcessedStore interface {
	Get(key ...string) interface{}
	Set(value interface{}, key ...string) error
}

// Manager is the struct that manages output sessions.
type Manager struct {
	opts   Opts
	logger hclog.Logger

	sessions   map[ID]*session
	sessionsMu sync.RWMutex

	lastID ID
}

func (s *Manager) SessionRoot(modelType string) (_ ID, lastProcessed interface{}, _ error) {
	return s.SessionFlex(false, modelType, 0, "", "")
}

func (s *Manager) SessionRootTracking(modelType string) (_ ID, lastProcessed interface{}, _ error) {
	return s.SessionFlex(true, modelType, 0, "", "")
}

func (s *Manager) Session(modelType string, parentSessionID ID, parentObjectID, parentObjectName string) (_ ID, lastProcessed interface{}, _ error) {
	return s.SessionFlex(false, modelType, parentSessionID, parentObjectID, parentObjectName)
}

func (s *Manager) SessionTracking(modelType string, parentSessionID ID, parentObjectID, parentObjectName string) (_ ID, lastProcessed interface{}, _ error) {
	return s.SessionFlex(true, modelType, parentSessionID, parentObjectID, parentObjectName)
}

func (s *Manager) SessionFlex(isTracking bool, name string, parentSessionID ID, parentObjectID, parentObjectName string) (_ ID, lastProcessed interface{}, _ error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	var parent *session
	if parentSessionID != 0 {
		var err error
		parent, err = s.get(parentSessionID)
		if err != nil {
			return 0, lastProcessed, err
		}
	}

	id := s.newID()
	sess := newSession(isTracking, name, id, s.opts.NewWriter, s.opts.SendProgress, parent, parentObjectID, parentObjectName)
	s.sessions[id] = sess

	if s.opts.LastProcessed != nil {
		lastProcessed = s.opts.LastProcessed.Get(sess.LastProcessedKey())
	}

	//s.logger.Info("create session", "type", modelType, "last_processed_old", lastProcessed)
	return id, lastProcessed, nil
}

func (s *Manager) newID() ID {
	s.lastID++
	return s.lastID
}

func (s *Manager) Write(id ID, objs []map[string]interface{}) error {
	sess, err := s.getLocked(id)
	if err != nil {
		return err
	}
	return sess.Write(s.logger, objs)
}

func (s *Manager) Progress(id ID, current, total int) {
	sess, err := s.getLocked(id)
	if err != nil {
		s.logger.Error("could not get session to update progress info", "err", err)
	}
	sess.Progress(current, total)
}

func (s *Manager) get(id ID) (*session, error) {
	res := s.sessions[id]
	if res == nil {
		return nil, fmt.Errorf("could not find session by id: %v", id)
	}
	return res, nil
}

func (s *Manager) getLocked(id ID) (*session, error) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	return s.get(id)
}

// GetModelType returnes modelType used for session
func (s *Manager) GetModelType(id ID) string {
	sess, err := s.getLocked(id)
	if err != nil {
		s.logger.Error("could not get session to get GetModelType", "err", err)
		return ""
	}
	if !sess.isTracking {
		return sess.name
	}
	return ""
}

// Done closes the session
func (s *Manager) Done(id ID, lastProcessed interface{}) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	sess, err := s.get(id)
	if err != nil {
		return err
	}

	if s.opts.SendProgressDone != nil {
		s.opts.SendProgressDone(sess.ProgressPath)
	}

	err = sess.Close()
	delete(s.sessions, id)
	if err != nil {
		return err
	}
	if s.opts.LastProcessed != nil {
		err = s.opts.LastProcessed.Set(lastProcessed, sess.LastProcessedKey())
		if err != nil {
			return err
		}
	}
	//s.logger.Info("session done", "type", modelType, "last_processed_new", lastProcessed)
	return nil
}
