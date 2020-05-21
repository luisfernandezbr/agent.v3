package jiracommon

import (
	"github.com/pinpt/integration-sdk/work"

	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
)

func projectsToCommon(projects []Project) (res []repoprojects.RepoProject) {
	for _, r := range projects {
		res = append(res, r)
	}
	return
}

func commonToProjects(common []repoprojects.RepoProject) (res []Project) {
	for _, r := range common {
		res = append(res, r.(Project))
	}
	return
}

func (s *JiraCommon) ProcessAllProjectsUsingExclusions(projectSender *objsender.Session, allProjectsDetailed []*work.Project) (notExcluded []Project, rerr error) {

	var allProjects []Project
	for _, obj := range allProjectsDetailed {
		p := Project{}
		p.JiraID = obj.RefID
		p.Key = obj.Identifier
		allProjects = append(allProjects, p)
	}

	res := repoprojects.Filter(s.opts.Logger, projectsToCommon(allProjects), repoprojects.FilterConfig{
		OnlyIncludeReadableIDs: s.opts.Projects,
		ExcludedIDs:            s.opts.ExcludedProjects,
		IncludedIDs:            s.opts.IncludedProjects,
	})

	notExcluded = commonToProjects(res)

	err := s.sendProjects(projectSender, allProjectsDetailed, notExcluded)
	if err != nil {
		rerr = err
		return
	}

	return
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
