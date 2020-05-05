package objsender

import (
	"sync"

	"github.com/pinpt/agent/rpcdef"
)

type SessionsWebhook struct {
	Data rpcdef.MutatedObjects
	mu   sync.Mutex
}

func NewSessionsWebhook() *SessionsWebhook {
	res := &SessionsWebhook{}
	res.Data = rpcdef.MutatedObjects{}
	return res
}

func (s *SessionsWebhook) write(objectType string, obj Model) {
	s.mu.Lock()
	s.mu.Unlock()
	s.Data[objectType] = append(s.Data[objectType], obj.ToMap())
}

func (s *SessionsWebhook) NewSession(objectType string) *SessionWebhook {
	return &SessionWebhook{objectType: objectType, sessions: s}
}

type SessionWebhook struct {
	objectType string
	sessions   *SessionsWebhook
}

func (s *SessionWebhook) Send(obj Model) error {
	s.sessions.write(s.objectType, obj)
	return nil
}

func (s *SessionWebhook) Done() error {
	return nil
}
