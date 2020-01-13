package main

import (
	"context"

	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/rpcdef"

	"github.com/hashicorp/go-hclog"
)

type Integration struct {
	logger hclog.Logger
	agent  rpcdef.Agent
}

func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	return nil
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	// queue one real repo for fetching
	gitFetch := rpcdef.GitRepoFetch{}
	gitFetch.RepoID = "r1"
	gitFetch.RefType = "github"
	gitFetch.URL = "https://github.com/pinpt/test_repo.git"
	gitFetch.UniqueName = "repo1"
	gitFetch.CommitURLTemplate = "#@@@sha@@@"
	gitFetch.BranchURLTemplate = "#@@@branch@@@"
	err := s.agent.ExportGitRepo(gitFetch)
	if err != nil {
		return res, err
	}

	res.Projects = s.exportRepoObjects()
	return res, nil
}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {
	res.Errors = append(res.Errors, "example validation error")
	return res, nil
}

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}
