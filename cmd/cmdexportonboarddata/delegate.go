package cmdexportonboarddata

import (
	"github.com/pinpt/agent.next/rpcdef"
)

type agentDelegate struct {
	export *export
}

func (s agentDelegate) ExportStarted(modelType string) (sessionID string, lastProcessed interface{}) {
	panic("not implemented export onboard data")
}

func (s agentDelegate) ExportDone(sessionID string, lastProcessed interface{}) {
	panic("not implemented export onboard data")
}

func (s agentDelegate) SendExported(sessionID string, objs []rpcdef.ExportObj) {
	panic("not implemented export onboard data")
}

func (s agentDelegate) ExportGitRepo(fetch rpcdef.GitRepoFetch) {
	panic("not implemented export onboard data")
}
