package oauthtoken

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/rpcdef"
)

type Manager struct {
	logger hclog.Logger
	agent  rpcdef.Agent
	token  string
	mu     sync.Mutex

	lastUpdate time.Time
}

func New(logger hclog.Logger, agent rpcdef.Agent) (*Manager, error) {
	s := &Manager{}
	s.logger = logger
	s.agent = agent
	err := s.Refresh()
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Get returns current auth token
// Safe for concurrent use
func (s *Manager) Get() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

// Refresh gets a new auth token, call this once if getting 401 when calling api
// Safe for concurrent use
func (s *Manager) Refresh() error {
	s.logger.Info("getting new oauth access token")
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastUpdate) < time.Minute {
		s.logger.Debug("was requested to refresh token, but last update was <1m ago, not refreshing, normal occurence if multiple threads are waiting for lock in token refresh")
		return nil
	}
	token, err := s.agent.OAuthNewAccessToken()
	if err != nil {
		return fmt.Errorf("could not get oauth access token based on refresh token, err %v", err)
	}
	s.token = token
	s.lastUpdate = time.Now()
	return nil
}
