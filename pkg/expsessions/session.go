package expsessions

import (
	"errors"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/expin"
)

type session struct {
	isTracking   bool
	name         string
	id           ID
	newWriter    NewWriterFunc
	sendProgress SendProgressFunc
	parent       *session

	ProgressPath ProgressPath

	writer Writer
	mu     sync.Mutex

	current int
	total   int

	written int
}

func newSession(
	export expin.Export,
	isTracking bool,
	name string,
	id ID,
	newWriter NewWriterFunc,
	sendProgress SendProgressFunc,
	parent *session,
	parentObjectID string,
	parentObjectName string) *session {
	s := &session{}
	s.isTracking = isTracking
	s.name = name
	s.id = id
	s.newWriter = newWriter
	s.sendProgress = sendProgress
	s.parent = parent

	if s.parent != nil {
		if parentObjectID == "" {
			panic("parentObjectID must be set if using parent session")
		}
		s.ProgressPath = s.parent.ProgressPath.Copy()
		s.ProgressPath = append(s.ProgressPath, ProgressPathComponent{
			ObjectID:   parentObjectID,
			ObjectName: parentObjectName})
	} else {
		s.ProgressPath = append(s.ProgressPath, ProgressPathComponent{
			TrackingName: export.String()})
	}

	if s.isTracking {
		s.ProgressPath = append(s.ProgressPath, ProgressPathComponent{
			TrackingName: name})
	} else {
		s.ProgressPath = append(s.ProgressPath, ProgressPathComponent{
			ModelName: name})
	}
	return s
}

func (s *session) LastProcessedKey() string {
	return s.ProgressPath.String()
}

func (s *session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isTracking {
		s.progress(s.written, s.written)
	}

	if s.writer == nil {
		// there was no stream, since no objects were sent
		return nil
	}
	return s.writer.Close()
}

func (s *session) Rollback() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.writer == nil {
		// there was no stream, since no objects were sent
		return nil
	}

	return s.writer.Rollback()
}

func (s *session) Write(logger hclog.Logger, objs []map[string]interface{}) error {
	if s.isTracking {
		return errors.New("tracking sessions do not support write method")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.written += len(objs)
	if s.writer == nil {
		s.writer = s.newWriter(s.name, s.id)
	}
	return s.writer.Write(logger, objs)
}

func (s *session) Progress(current, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress(current, total)
}

func (s *session) progress(current, total int) {
	//s.current = current
	//s.total = total
	if s.sendProgress != nil {
		s.sendProgress(s.ProgressPath, current, total)
	}
}
