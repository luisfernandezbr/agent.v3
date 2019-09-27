package objsender2

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pinpt/agent.next/rpcdef"
)

type Model interface {
	ToMap() map[string]interface{}
}

type Session struct {
	isTracking bool
	agent      rpcdef.Agent
	startTime  time.Time
	name       string

	sessionID     int
	lastProcessed interface{}

	batch *batch

	progressMu sync.Mutex
	current    int
	total      int

	lastProgressUpdate time.Time

	noAutoProgress bool
}

func Root(agent rpcdef.Agent, refType string) (*Session, error) {
	s := &Session{}
	s.agent = agent
	s.startTime = time.Now()
	s.name = refType
	err := s.start()
	if err != nil {
		return nil, err
	}
	return s, nil
}

type SessionTracking struct {
	Session
}

func RootTracking(agent rpcdef.Agent, trackingName string) (*Session, error) {
	s := &Session{}
	s.agent = agent
	s.startTime = time.Now()
	s.name = trackingName
	s.isTracking = true
	err := s.start()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (par *Session) Session(refType string, parentObjectID string, parentObjectName string) (*Session, error) {
	s := &Session{}
	s.agent = par.agent
	s.startTime = time.Now()
	s.name = refType

	var err error
	s.sessionID, s.lastProcessed, err = s.agent.SessionStart(s.isTracking, s.name, par.sessionID, parentObjectID, parentObjectName)
	if err != nil {
		return nil, err
	}
	s.batch = &batch{sessionID: s.sessionID, agent: s.agent}
	return s, nil
}

func (par *Session) SessionTracking(refType string, parentObjectID string, parentObjectName string) (*Session, error) {
	s := &Session{}
	s.isTracking = true
	s.agent = par.agent
	s.startTime = time.Now()
	s.name = refType

	var err error
	s.sessionID, s.lastProcessed, err = s.agent.SessionStart(s.isTracking, s.name, par.sessionID, parentObjectID, parentObjectName)
	if err != nil {
		return nil, err
	}
	s.batch = &batch{sessionID: s.sessionID, agent: s.agent}
	return s, nil
}

func (s *Session) start() error {
	var err error
	s.sessionID, s.lastProcessed, err = s.agent.SessionStart(s.isTracking, s.name, 0, "", "")
	if err != nil {
		return err
	}
	s.batch = &batch{sessionID: s.sessionID, agent: s.agent}
	return nil
}

func (s *Session) SetTotal(total int) error {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	if total == s.total {
		return nil
	}
	s.total = total
	s.lastProgressUpdate = time.Now()
	err := s.agent.SessionProgress(s.sessionID, s.current, s.total)
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) Send(obj Model) error {
	return s.SendMap(obj.ToMap())
}

func (s *Session) SetNoAutoProgress(v bool) {
	s.noAutoProgress = true
}

func (s *Session) SendMap(m map[string]interface{}) error {
	if s.isTracking {
		return errors.New("send is not supported for tracking sessions")
	}
	if !s.noAutoProgress {
		err := s.incProgress()
		if err != nil {
			return err
		}
	}
	return s.batch.Send(m)
}

func (s *Session) IncProgress() error {
	if !s.isTracking && !s.noAutoProgress {
		return errors.New("IncProgress is only for tracking sessions or when SetNoAutoProgress = true")
	}
	return s.incProgress()
}

func (s *Session) incProgress() error {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.current++
	return s.sendProgressNoLock()
}

func (s *Session) sendProgressNoLock() error {
	if time.Since(s.lastProgressUpdate) > 1*time.Second {
		s.lastProgressUpdate = time.Now()
		err := s.agent.SessionProgress(s.sessionID, s.current, s.total)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) LastProcessedTime() time.Time {
	if s.lastProcessed == nil {
		return time.Time{}
	}
	str, ok := s.lastProcessed.(string)
	if !ok {
		panic(fmt.Errorf("attempted to get last processed time as string, but have different type stored %v %T", s.lastProcessed, s.lastProcessed))
	}
	res, err := time.Parse(time.RFC3339, str)
	if err != nil {
		panic(fmt.Errorf("attempted to parse last processed time, got err %v %v", s.lastProcessed, err))
	}
	return res
}

func (s *Session) Done() error {
	err := s.batch.Flush()
	if err != nil {
		return err
	}
	s.agent.ExportDone(strconv.Itoa(s.sessionID), s.startTime.Format(time.RFC3339))
	return nil
}
