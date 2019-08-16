package cmdvalidateconfig

import (
	"github.com/pinpt/agent.next/rpcdef"
)

type agentDelegate struct {
	validator *validator
}

func (s agentDelegate) ExportStarted(modelType string) (sessionID string, lastProcessed interface{}) {
	panic("not implemented in validate")
}

func (s agentDelegate) ExportDone(sessionID string, lastProcessed interface{}) {
	panic("not implemented in validate")
}

func (s agentDelegate) SendExported(sessionID string, objs []rpcdef.ExportObj) {
	panic("not implemented in validate")
}

func (s agentDelegate) ExportGitRepo(fetch rpcdef.GitRepoFetch) {
	panic("not implemented in validate")
}
