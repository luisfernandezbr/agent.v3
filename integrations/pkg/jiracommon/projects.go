package jiracommon

import (
	"fmt"

	"github.com/pinpt/integration-sdk/work"

	"github.com/pinpt/agent.next/pkg/objsender"
)

func (s *JiraCommon) ProcessAllProjectsUsingExclusions(allProjectsDetailed []*work.Project) (notExcluded []Project, _ error) {
	var allProjects []Project
	for _, obj := range allProjectsDetailed {
		p := Project{}
		p.JiraID = obj.RefID
		p.Key = obj.Identifier
		allProjects = append(allProjects, p)
	}
	projects, err := s.getProjectsFilterExcluded(allProjects)
	if err != nil {
		return nil, err
	}
	err = s.sendProjects(allProjectsDetailed, projects)
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (s *JiraCommon) getProjectsFilterExcluded(all []Project) ([]Project, error) {
	allm := map[string]bool{}
	for _, p := range all {
		allm[p.JiraID] = true
	}
	excluded := map[string]bool{}
	for _, id := range s.opts.ExcludedProjects {
		if !allm[id] {
			return nil, fmt.Errorf("wanted to exclude non existing repo: %v", id)
		}
		excluded[id] = true
	}

	filtered := map[string]Project{}
	for _, p := range all {
		if excluded[p.JiraID] {
			continue
		}
		filtered[p.JiraID] = p
	}

	s.opts.Logger.Info("projects", "found", len(all), "excluded_definition", len(s.opts.ExcludedProjects), "result", len(filtered))

	res := []Project{}
	for _, p := range filtered {
		res = append(res, p)
	}
	return res, nil
}

func (s *JiraCommon) sendProjects(allProjects []*work.Project, notExcluded []Project) error {
	sender := objsender.NewNotIncremental(s.agent, work.ProjectModelName.String())
	defer sender.Done()

	ok := map[string]bool{}
	for _, p := range notExcluded {
		ok[p.JiraID] = true
	}

	var res2 []objsender.Model
	for _, obj := range allProjects {
		if !ok[obj.RefID] {
			continue
		}
		res2 = append(res2, obj)
	}
	err := sender.Send(res2)
	if err != nil {
		return err
	}
	return nil
}
