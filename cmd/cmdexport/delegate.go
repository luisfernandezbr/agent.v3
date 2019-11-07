package cmdexport

import (
	"github.com/pinpt/agent.next/pkg/expsessions"
	"github.com/pinpt/agent.next/pkg/integrationid"
	"github.com/pinpt/agent.next/rpcdef"
)

type agentDelegate struct {
	export     *export
	in         integrationid.ID
	expsession *expsessions.Manager
}

func newAgentDelegate(export *export, expsession *expsessions.Manager, in integrationid.ID) *agentDelegate {
	s := &agentDelegate{}
	s.export = export
	s.expsession = expsession
	s.in = in
	return s
}

func (s agentDelegate) ExportStarted(modelType string) (sessionID string, lastProcessed interface{}) {
	sessionID, lastProcessed, err := s.export.sessions.new(s.in, modelType)
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

func (s agentDelegate) SessionStart(isTracking bool, name string, parentSessionID int, parentObjectID, parentObjectName string) (sessionID int, lastProcessed interface{}, _ error) {
	id, lastProcessed, err := s.expsession.SessionFlex(s.in, isTracking, name, expsessions.ID(parentSessionID), parentObjectID, parentObjectName)
	if err != nil {
		return 0, nil, err
	}
	return int(id), lastProcessed, nil
}

func (s agentDelegate) SessionProgress(id int, current, total int) error {
	s.expsession.Progress(expsessions.ID(id), current, total)
	return nil
}

func (s agentDelegate) OAuthNewAccessToken() (token string, _ error) {
	return s.export.OAuthNewAccessToken(s.in.Name)
}

func (s agentDelegate) SendPauseEvent(msg string) error {
	return s.export.SendPauseEvent(msg)
}

func (s agentDelegate) SendContinueEvent(msg string) error {
	return s.export.SendContinueEvent(msg)
}
