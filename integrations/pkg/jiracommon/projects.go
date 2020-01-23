package jiracommon

import (
	"fmt"

	"github.com/pinpt/integration-sdk/work"

	"github.com/pinpt/agent/integrations/pkg/objsender"
)

func (s *JiraCommon) ProcessAllProjectsUsingExclusions(projectSender *objsender.Session, allProjectsDetailed []*work.Project) (notExcluded []Project, rerr error) {
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
			rerr = err
			return
		}
	} else {
		var err error
		projects, err = s.getProjectsFilterExcluded(allProjects)
		if err != nil {
			rerr = err
			return
		}
	}

	err := s.sendProjects(projectSender, allProjectsDetailed, projects)
	if err != nil {
		rerr = err
		return
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

	var included []Project
	{
		onlyInclude := s.opts.IncludedProjects

		ok := map[string]bool{}
		for _, id := range onlyInclude {
			ok[id] = true
		}
		for _, p := range all {
			if !ok[p.JiraID] {
				continue
			}
			included = append(included, p)
		}

	}

	filtered := map[string]Project{}
	for _, p := range included {
		if excluded[p.JiraID] {
			continue
		}
		filtered[p.JiraID] = p
	}

	s.opts.Logger.Info("projects", "found", len(all), "excluded_definition", len(s.opts.ExcludedProjects), "included_definition", len(s.opts.IncludedProjects), "result", len(filtered))

	res := []Project{}
	for _, p := range filtered {
		res = append(res, p)
	}
	return res, nil
}

func (s *JiraCommon) sendProjects(sender *objsender.Session, allProjects []*work.Project, notExcluded []Project) error {

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
