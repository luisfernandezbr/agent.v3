package cmdexport

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/exportrepo"
	"github.com/pinpt/agent/pkg/integrationid"
	"github.com/pinpt/agent/rpcdef"
)

type Result struct {
	Integrations []ResultIntegration `json:"integrations"`
}

type ResultIntegration struct {
	ID       string          `json:"id"`
	Error    string          `json:"error"`
	Projects []ResultProject `json:"projects"`
	Duration time.Duration   `json:"duration"`
}

type ResultProject struct {
	rpcdef.ExportProject
	HasGitRepo bool   `json:"has_git_repo"`
	GitError   string `json:"git_error"`
}

func (s *export) handleIntegrationPanics(res map[integrationid.ID]runResult) {
	s.Logger.Info("Checking integrations for panics")
	for id, integration := range s.Integrations {
		ires := res[id]
		if ires.Err != nil {
			s.Logger.Error("Export failed in integration", "integration", id, "err", ires.Err)
			if err := s.Command.CloseOnlyIntegrationAndHandlePanic(integration); err != nil {
				s.Logger.Error("Could not close integration", "err", err)
			}
			continue
		}
		s.Logger.Info("Export success for integration", "integration", id)
	}
}

// formatResults links git errors with integration errors and returns them as Result
func (s *export) formatResults(runResult map[integrationid.ID]runResult) Result {
	gitResults := s.gitResults

	resAll := Result{}
	for id, res0 := range runResult {
		res := ResultIntegration{}
		res.ID = id.String()
		if res0.Err != nil {
			res.Error = res0.Err.Error()
		}
		res.Duration = res0.Duration
		for _, project0 := range res0.Res.Projects {
			project := ResultProject{}
			project.ExportProject = project0
			gitErr, ok := gitResults[id][project.ID]
			if ok {
				project.HasGitRepo = true
				if gitErr != nil {
					if gitErr == exportrepo.ErrRevParseFailed {
						project.GitError = "empty_repo"
					} else {
						project.GitError = gitErr.Error()
					}
				}
			}
			res.Projects = append(res.Projects, project)
		}
		resAll.Integrations = append(resAll.Integrations, res)
	}

	return resAll
}

func (s Result) Log(logger hclog.Logger) {
	logger.Info("Printing export results")

	for _, integration := range s.Integrations {
		prefix := "Integration " + integration.ID + " "
		logger.Info(prefix, "duration", integration.Duration)
		if integration.Error != "" {
			logger.Error(prefix+"failed with error", "err", integration.Error)
			continue
		}
		if len(integration.Projects) == 0 {
			logger.Warn(prefix + "returned no errors, but no projects were processed")
			continue
		}
		total := len(integration.Projects)
		success := 0
		failed := 0
		gitFailed := 0
		for _, pro := range integration.Projects {
			if pro.Error != "" {
				failed++
			} else if pro.GitError != "" {
				gitFailed++
			} else {
				success++
			}
		}
		if success == total {
			logger.Info(prefix+"completed with no errors", "projects", total)
			continue
		}
		logger.Error(prefix+"failed on some projects", "total", total, "success", success, "integration_failed", failed, "git_failed", gitFailed)
		i := 0
		for _, project := range integration.Projects {
			if project.Error == "" && project.GitError == "" {
				continue
			}
			i++
			if i > 10 {
				logger.Error(prefix + "failed on more than 10 projects, skipping output")
				continue
			}
			logger.Error(prefix+"failed on project", "id", project.ID, "ref_id", project.RefID, "readable_id", project.ReadableID, "err", project.Error, "git_error", project.GitError)
		}
	}
}

func (s Result) FailInCaseOfIntegrationErrors() error {
	for _, integration := range s.Integrations {
		if integration.Error != "" {
			return fmt.Errorf("Failed export. Integration %v returned error: %v", integration.ID, integration.Error)
		}
	}
	return nil
}
