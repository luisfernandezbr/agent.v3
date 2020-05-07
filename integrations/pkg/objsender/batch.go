package objsender

import (
	"strconv"
	"sync"

	"github.com/pinpt/agent/rpcdef"
)

type batch struct {
	batch     []rpcdef.ExportObj
	mu        sync.Mutex
	sessionID int
	agent     rpcdef.Agent
}

const maxBatch = 2

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
	s.agent.SendExported(strconv.Itoa(s.sessionID), data)
	return nil
}

func (s *batch) Flush() error {
	s.mu.Lock()
	data := s.batch
	s.batch = []rpcdef.ExportObj{}
	s.mu.Unlock()
	return s.flushNoLock(data)
}
