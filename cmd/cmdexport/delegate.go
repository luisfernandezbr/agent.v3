package cmdexport

import (
	"github.com/pinpt/agent.next/rpcdef"
)

type agentDelegate struct {
	export          *export
	integrationName string
}

func newAgentDelegate(export *export, integrationName string) *agentDelegate {
	s := &agentDelegate{}
	s.export = export
	s.integrationName = integrationName
	return s
}

func (s agentDelegate) ExportStarted(modelType string) (sessionID string, lastProcessed interface{}) {
	sessionID, lastProcessed, err := s.export.sessions.new(modelType)
	if err != nil {
		panic(err)
	}
	return sessionID, lastProcessed
}

func (s agentDelegate) ExportDone(sessionID string, lastProcessed interface{}) {
	err := s.export.sessions.ExportDone(sessionID, lastProcessed)
	if err != nil {
		panic(err)
	}
}

func (s agentDelegate) SendExported(sessionID string, objs []rpcdef.ExportObj) {
	err := s.export.sessions.Write(sessionID, objs)
	if err != nil {
		panic(err)
	}
}

func (s agentDelegate) ExportGitRepo(fetch rpcdef.GitRepoFetch) error {
	s.export.gitProcessingRepos <- fetch
	return nil
}

func (s agentDelegate) OAuthNewAccessToken() (token string, _ error) {
	return s.export.OAuthNewAccessToken(s.integrationName)
}
