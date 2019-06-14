package cmdexport

import (
	"fmt"

	"github.com/pinpt/agent2/rpcdef"
)

type agentDelegate struct {
	export *export
}

func (s agentDelegate) ExportStarted(modelType string) (sessionID string) {
	return s.export.sessions.new(modelType)
}

func (s agentDelegate) ExportDone(sessionID string) {

}

func (s agentDelegate) SendExported(sessionID string, lastProcessedToken string, objs []rpcdef.ExportObj) {
	session := s.export.sessions.get(sessionID)
	modelType := session.modelType
	fmt.Println("agent: SendExported ", modelType, "len(objs)=", len(objs))
}

func (s agentDelegate) ExportGitRepo(fetch rpcdef.GitRepoFetch) {
	fmt.Println("agent: ExportGitRepo called")
}
