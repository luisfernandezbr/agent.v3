package cmdexport

import (
	"fmt"

	"github.com/pinpt/agent.next/rpcdef"
)

type agentDelegate struct {
	export *export
}

func (s agentDelegate) ExportStarted(modelType string) (sessionID string) {
	id, err := s.export.sessions.new(modelType)
	if err != nil {
		panic(err)
	}
	return id
}

func (s agentDelegate) ExportDone(sessionID string) {
	err := s.export.sessions.Close(sessionID)
	if err != nil {
		panic(err)
	}
}

func (s agentDelegate) SendExported(sessionID string, lastProcessedToken string, objs []rpcdef.ExportObj) {
	err := s.export.sessions.Write(sessionID, objs)
	if err != nil {
		panic(err)
	}
}

func (s agentDelegate) ExportGitRepo(fetch rpcdef.GitRepoFetch) {
	fmt.Println("agent: ExportGitRepo called")
}
