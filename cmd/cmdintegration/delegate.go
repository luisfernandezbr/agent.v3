package cmdintegration

import (
	"time"

	"github.com/pinpt/agent/pkg/integrationid"
	"github.com/pinpt/agent/rpcdef"
)

type AgentDelegateMinimal interface {
	OAuthNewAccessToken(integrationName string) (token string, _ error)
}

type agentDelegate struct {
	min AgentDelegateMinimal
	in  integrationid.ID
}

func AgentDelegateMinFactory(min AgentDelegateMinimal) func(in integrationid.ID) rpcdef.Agent {
	return func(in integrationid.ID) rpcdef.Agent {
		return &agentDelegate{min: min, in: in}
	}
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

func (s agentDelegate) ExportGitRepo(fetch rpcdef.GitRepoFetch) error {
	panic("not implemented")
}

func (s agentDelegate) SessionStart(isTracking bool, name string, parentSessionID int, parentObjectID, parentObjectName string) (sessionID int, lastProcessed interface{}, _ error) {
	panic("not implemented")
}

func (s agentDelegate) SessionProgress(id int, current, total int) error {
	panic("not implemented")
}

func (s agentDelegate) OAuthNewAccessToken() (token string, _ error) {
	return s.min.OAuthNewAccessToken(s.in.Name)
}

func (s agentDelegate) SendPauseEvent(msg string, resumeDate time.Time) error {
	return nil
}

func (s agentDelegate) SendResumeEvent(msg string) error {
	return nil
}
