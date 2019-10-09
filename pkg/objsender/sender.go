package objsender

import (
	"fmt"
	"sync"
	"time"

	"github.com/pinpt/agent.next/rpcdef"
)

type batch struct {
	batch     []rpcdef.ExportObj
	mu        sync.Mutex
	sessionID string
	agent     rpcdef.Agent
}

const maxBatch = 10

func (s *batch) Send(m map[string]interface{}) error {
	s.mu.Lock()
	s.batch = append(s.batch, rpcdef.ExportObj{Data: m})
	if len(s.batch) >= maxBatch {
		data := s.batch
		s.batch = []rpcdef.ExportObj{}
		s.mu.Unlock()
		return s.flushNoLock(data)
	}
	s.mu.Unlock()
	return nil
}

func (s *batch) flushNoLock(data []rpcdef.ExportObj) error {
	s.agent.SendExported(s.sessionID, data)
	return nil
}

func (s *batch) Flush() error {
	s.mu.Lock()
	data := s.batch
	s.batch = []rpcdef.ExportObj{}
	s.mu.Unlock()
	return s.flushNoLock(data)
}

type Sender interface {
	Send(obj Model) error
	SendMap(m map[string]interface{}) error
	Done() error
}

type NotIncremental struct {
	RefType   string
	SessionID string
	agent     rpcdef.Agent
	batch     *batch
}

func NewNotIncremental(agent rpcdef.Agent, refType string) *NotIncremental {
	if agent == nil {
		panic("provide agent")
	}
	s := &NotIncremental{}
	s.RefType = refType
	s.agent = agent
	s.SessionID, _ = s.agent.ExportStarted(s.RefType)
	s.batch = &batch{sessionID: s.SessionID, agent: s.agent}
	return s
}

type Model interface {
	ToMap() map[string]interface{}
}

func (s *NotIncremental) Send(obj Model) error {
	return s.batch.Send(obj.ToMap())
}

func (s *NotIncremental) SendMap(m map[string]interface{}) error {
	return s.batch.Send(m)
}

func (s *NotIncremental) Done() error {
	err := s.batch.Flush()
	if err != nil {
		return err
	}
	s.agent.ExportDone(s.SessionID, nil)
	return nil
}

type IncrementalDateBased struct {
	RefType       string
	SessionID     string
	agent         rpcdef.Agent
	StartTime     time.Time
	LastProcessed time.Time

	batch *batch
}

func NewIncrementalDateBased(agent rpcdef.Agent, refType string) (*IncrementalDateBased, error) {
	if agent == nil {
		panic("provide agent")
	}
	s := &IncrementalDateBased{}
	s.agent = agent
	s.StartTime = time.Now()
	s.RefType = refType
	sessionID, lastProcessed := s.agent.ExportStarted(s.RefType)
	if lastProcessed != nil {
		lp, err := time.Parse(time.RFC3339, lastProcessed.(string))
		if err != nil {
			return nil, fmt.Errorf("last processed timestamp is not valid, err: %v", err)
		}
		s.LastProcessed = lp
	}
	s.SessionID = sessionID
	s.agent = agent
	s.batch = &batch{sessionID: s.SessionID, agent: s.agent}
	return s, nil
}

func (s *IncrementalDateBased) Send(obj Model) error {
	return s.batch.Send(obj.ToMap())
}

func (s *IncrementalDateBased) SendMap(m map[string]interface{}) error {
	return s.batch.Send(m)
}

func (s *IncrementalDateBased) Done() error {
	err := s.batch.Flush()
	if err != nil {
		return err
	}
	s.agent.ExportDone(s.SessionID, s.StartTime.Format(time.RFC3339))
	return nil
}
