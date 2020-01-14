package cmdexport

import (
	"time"

	"github.com/pinpt/agent/pkg/expin"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/rpcdef"
)

type agentDelegate struct {
	export     *export
	expin      expin.Export
	expsession *expsessions.Manager
}

func newAgentDelegate(export *export, expsession *expsessions.Manager, expin expin.Export) *agentDelegate {
	s := &agentDelegate{}
	s.export = export
	s.expsession = expsession
	s.expin = expin
	return s
}

func (s agentDelegate) ExportStarted(modelType string) (sessionID string, lastProcessed interface{}) {
	sessionID, lastProcessed, err := s.export.sessions.new(s.expin, modelType)
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
	fetch2 := gitRepoFetch{}
	fetch2.GitRepoFetch = fetch
	fetch2.ind = s.expin.Index
	s.export.gitProcessingRepos <- fetch2
	return nil
}

func (s agentDelegate) SessionStart(isTracking bool, name string, parentSessionID int, parentObjectID, parentObjectName string) (sessionID int, lastProcessed interface{}, _ error) {
	id, lastProcessed, err := s.expsession.SessionFlex(s.expin, isTracking, name, expsessions.ID(parentSessionID), parentObjectID, parentObjectName)
	if err != nil {
		return 0, nil, err
	}
	return int(id), lastProcessed, nil
}

func (s agentDelegate) SessionProgress(id int, current, total int) error {
	s.expsession.Progress(expsessions.ID(id), current, total)
	return nil
}

func (s agentDelegate) SessionRollback(id int) error {
	return s.export.sessions.Rollback(expsessions.ID(id))
}

func (s agentDelegate) OAuthNewAccessToken() (token string, _ error) {
	return s.export.OAuthNewAccessToken(s.expin.Index)
}

func (s agentDelegate) SendPauseEvent(msg string, resumeDate time.Time) error {
	return s.export.SendPauseEvent(s.expin, msg, resumeDate)
}

func (s agentDelegate) SendResumeEvent(msg string) error {
	return s.export.SendResumeEvent(s.expin, msg)
}
