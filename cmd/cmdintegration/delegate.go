package cmdintegration

import "github.com/pinpt/agent.next/rpcdef"

type AgentDelegateMinimal interface {
	OAuthNewAccessToken(integrationName string) (token string, _ error)
}

type agentDelegate struct {
	min             AgentDelegateMinimal
	integrationName string
}

func AgentDelegateMinFactory(min AgentDelegateMinimal) func(integrationName string) rpcdef.Agent {
	return func(integrationName string) rpcdef.Agent {
		return &agentDelegate{min: min, integrationName: integrationName}
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

func (s agentDelegate) OAuthNewAccessToken() (token string, _ error) {
	return s.min.OAuthNewAccessToken(s.integrationName)
}
