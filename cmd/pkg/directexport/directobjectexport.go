package directexport

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/expin"
	"github.com/pinpt/agent/rpcdef"
)

type AgentDelegate interface {
	OAuthNewAccessToken(ind expin.Export) (token string, _ error)
	ExportGitRepo(fetch rpcdef.GitRepoFetch) error
}

type agentDelegate struct {
	logger   hclog.Logger
	delegate AgentDelegate
	exp      expin.Export
}

func AgentDelegateFactory(logger hclog.Logger, delegate AgentDelegate) func(exp expin.Export) rpcdef.Agent {
	return func(exp expin.Export) rpcdef.Agent {
		return &agentDelegate{
			logger:   logger,
			delegate: delegate,
			exp:      exp}
	}
}

func (s agentDelegate) ExportGitRepo(fetch rpcdef.GitRepoFetch) error {
	return s.delegate.ExportGitRepo(fetch)
}

func (s agentDelegate) ExportStarted(modelType string) (sessionID string, lastProcessed interface{}) {
	panic("not implemented")
}

func (s agentDelegate) ExportDone(sessionID string, lastProcessed interface{}) {
	panic("not implemented")
}

func (s agentDelegate) SendExported(sessionID string, objs []rpcdef.ExportObj) {
	panic("not implemented")
}

func (s agentDelegate) SessionStart(isTracking bool, name string, parentSessionID int, parentObjectID, parentObjectName string) (sessionID int, lastProcessed interface{}, _ error) {
	panic("not implemented")
}

func (s agentDelegate) SessionProgress(id int, current, total int) error {
	panic("not implemented")
}

func (s agentDelegate) SessionRollback(id int) error {
	panic("not implemented")
}

func (s agentDelegate) OAuthNewAccessToken() (token string, _ error) {
	return s.delegate.OAuthNewAccessToken(s.exp)
}

func (s agentDelegate) SendPauseEvent(msg string, resumeDate time.Time) error {
	s.logger.Info("pausing integration due to throttling", "msg", msg, "integration", s.exp.String(), "duration", resumeDate.Sub(time.Now()).String())
	return nil
}

func (s agentDelegate) SendResumeEvent(msg string) error {
	s.logger.Info("continue with integration after throttling", "msg", msg, "integration", s.exp.String())

	return nil
}
