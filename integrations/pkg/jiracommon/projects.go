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
	var projects []Project
	if len(s.opts.Projects) != 0 {
		var err error
		projects, err = s.getProjectsOnlySpecified(allProjects)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		projects, err = s.getProjectsFilterExcluded(allProjects)
		if err != nil {
			return nil, err
		}
	}
	err := s.sendProjects(allProjectsDetailed, projects)
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (s *JiraCommon) getProjectsOnlySpecified(all []Project) (res []Project, _ error) {
	allm := map[string]Project{}
	for _, p := range all {
		allm[p.Key] = p
	}
	for _, key := range s.opts.Projects {
		p, ok := allm[key]
		if !ok {
			return nil, fmt.Errorf("wanted to process non existing project: %v", key)
		}
		res = append(res, p)
	}
	s.opts.Logger.Info("projects", "found", len(all), "directly_passed", len(s.opts.Projects))
	return
}

func (s *JiraCommon) getProjectsFilterExcluded(all []Project) ([]Project, error) {
	allm := map[string]bool{}
	for _, p := range all {
		allm[p.JiraID] = true
	}

	excluded := map[string]bool{}
	for _, id := range s.opts.ExcludedProjects {
		if !allm[id] {
			return nil, fmt.Errorf("wanted to exclude non existing project: %v", id)
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

	for _, obj := range allProjects {
		if !ok[obj.RefID] {
			continue
		}
		err := sender.Send(obj)
		if err != nil {
			return err
		}
	}
	return nil
}
