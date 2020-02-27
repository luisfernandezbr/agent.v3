package main

import (
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) onboardExportRepos() (res rpcdef.OnboardExportResult, err error) {

	var repos []*sourcecode.Repo
	_, repos, err = s.api.FetchAllRepos([]string{}, []string{}, []string{})
	if err != nil {
		s.logger.Error("error fetching repos for onboard export repos")
		return
	}
	var records []map[string]interface{}
	for _, repo := range repos {
		r := &agent.RepoResponseRepos{
			Description: repo.Description,
			Language:    repo.Language,
			Name:        repo.Name,
			RefID:       repo.RefID,
			RefType:     repo.RefType,
		}
		records = append(records, r.ToMap())
	}
	res.Data = records
	return
}

func (s *Integration) onboardExportProjects() (res rpcdef.OnboardExportResult, err error) {
	projects, err := s.api.FetchProjects([]string{}, []string{}, []string{})
	var records []map[string]interface{}
	for _, proj := range projects {
		resp := &agent.ProjectResponseProjects{
			Active:     proj.Active,
			Identifier: proj.Identifier,
			Name:       proj.Name,
			RefID:      proj.RefID,
			RefType:    proj.RefType,
			URL:        proj.URL,
		}
		records = append(records, resp.ToMap())
	}
	res.Data = records
	return res, err
}

func (s *Integration) onboardWorkConfig() (res rpcdef.OnboardExportResult, err error) {
	conf, err := s.api.FetchWorkConfig()
	if err != nil {
		res.Error = err
		return
	}
	res.Data = conf.ToMap()
	return
}
