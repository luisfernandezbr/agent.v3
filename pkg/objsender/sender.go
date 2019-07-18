package objsender

import (
	"fmt"
	"time"

	"github.com/pinpt/agent.next/rpcdef"
)

type NotIncremental struct {
	RefType   string
	SessionID string
	agent     rpcdef.Agent
}

func NewNotIncremental(agent rpcdef.Agent, refType string) *NotIncremental {
	if agent == nil {
		panic("provide agent")
	}
	s := &NotIncremental{}
	s.RefType = refType
	s.agent = agent
	s.SessionID, _ = s.agent.ExportStarted(s.RefType)
	return s
}

type Model interface {
	ToMap(...bool) map[string]interface{}
}

func (s *NotIncremental) Send(objs []Model) error {
	if len(objs) == 0 {
		return nil
	}
	var objs2 []rpcdef.ExportObj
	for _, obj := range objs {
		objs2 = append(objs2, rpcdef.ExportObj{Data: obj.ToMap()})
	}
	s.agent.SendExported(s.SessionID, objs2)
	return nil
}

func (s *NotIncremental) SendMaps(data []map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}
	var objs2 []rpcdef.ExportObj
	for _, obj := range data {
		objs2 = append(objs2, rpcdef.ExportObj{Data: obj})
	}
	s.agent.SendExported(s.SessionID, objs2)
	return nil
}

func (s *NotIncremental) Done() error {
	s.agent.ExportDone(s.SessionID, nil)
	return nil
}

type IncrementalDateBased struct {
	RefType       string
	SessionID     string
	agent         rpcdef.Agent
	StartTime     time.Time
	LastProcessed time.Time
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
	return s, nil
}

func (s *IncrementalDateBased) Send(objs []Model) error {
	if len(objs) == 0 {
		return nil
	}
	var objs2 []rpcdef.ExportObj
	for _, obj := range objs {
		objs2 = append(objs2, rpcdef.ExportObj{Data: obj.ToMap()})
	}
	s.agent.SendExported(s.SessionID, objs2)
	return nil
}

func (s *IncrementalDateBased) SendMaps(data []map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}
	var objs2 []rpcdef.ExportObj
	for _, obj := range data {
		objs2 = append(objs2, rpcdef.ExportObj{Data: obj})
	}
	s.agent.SendExported(s.SessionID, objs2)
	return nil
}

func (s *IncrementalDateBased) Done() {
	s.agent.ExportDone(s.SessionID, s.StartTime.Format(time.RFC3339))
}
