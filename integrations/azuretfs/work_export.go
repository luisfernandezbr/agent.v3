package main

import (
	"fmt"

	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/pkg/integrationid"
	"github.com/pinpt/agent/rpcdef"

	"github.com/pinpt/integration-sdk/work"
)

func (s *Integration) exportWork() (exportResults []rpcdef.ExportProject, rerr error) {

	var orgname string
	if s.Creds.Organization != nil {
		orgname = *s.Creds.Organization
	} else {
		orgname = *s.Creds.CollectionName
	}
	sender, err := s.orgSession.Session(work.ProjectModelName.String(), orgname, orgname)
	if err != nil {
		rerr = err
		return
	}

	projectsDetails, err := s.api.FetchProjects(s.Projects, s.ExcludedProjectIDs, s.IncludedProjectIDs)
	if err != nil {
		rerr = err
		return
	}
	if err := sender.SetTotal(len(projectsDetails)); err != nil {
		rerr = err
		return
	}

	sender.SetNoAutoProgress(true)

	for _, proj := range projectsDetails {
		if err := sender.Send(proj); err != nil {
			rerr = err
			return
		}
	}

	var projects []Project
	for _, project := range projectsDetails {
		projects = append(projects, Project{RefID: project.RefID, Name: project.Name})
	}
	var projectsIface []repoprojects.RepoProject
	for _, project := range projects {
		projectsIface = append(projectsIface, project)
	}

	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.logger
	processOpts.ProjectFn = func(ctx *repoprojects.ProjectCtx) error {
		proj := ctx.Project.(Project)
		teamids, err := s.api.FetchTeamIDs(proj.RefID)
		if err != nil {
			return err
		}
		if err = s.processWorkUsers(ctx, proj, teamids); err != nil {
			return err
		}
		if err = s.processWorkItems(ctx, proj); err != nil {
			return err
		}
		if err = s.processSprints(ctx, proj, teamids); err != nil {
			return err
		}
		return nil
	}

	processOpts.Concurrency = s.Concurrency
	processOpts.Projects = projectsIface
	processOpts.IntegrationType = integrationid.TypeSourcecode
	processOpts.CustomerID = s.customerid
	processOpts.RefType = s.RefType.String()
	processOpts.Sender = sender

	processor := repoprojects.NewProcess(processOpts)
	exportResults, err = processor.Run()
	if err != nil {
		rerr = err
		return
	}

	err = sender.Done()
	if err != nil {
		rerr = err
	}
	return
}

type Project struct {
	RefID string
	Name  string
}

func (s Project) GetID() string {
	return s.RefID
}

func (s Project) GetReadableID() string {
	return s.Name
}

func (s *Integration) processWorkUsers(ctx *repoprojects.ProjectCtx, proj Project, teamids []string) error {
	sender, err := ctx.Session(work.UserModelName)
	if err != nil {
		return err
	}
	users, err := s.api.FetchWorkUsers(proj.RefID, teamids)
	if err != nil {
		return err
	}
	if err := sender.SetTotal(len(users)); err != nil {
		return err
	}
	for _, user := range users {
		if err := sender.Send(user); err != nil {
			return err
		}
	}
	return nil
}

func (s *Integration) processWorkItems(ctx *repoprojects.ProjectCtx, proj Project) error {
	sender, err := ctx.Session(work.IssueModelName)
	if err != nil {
		return err
	}
	allids, err := s.api.FetchItemIDs(proj.RefID, sender.LastProcessedTime())
	if err != nil {
		return err
	}
	if err := sender.SetTotal(len(allids)); err != nil {
		s.logger.Error("error setting total ids on processWorkItems", "err", err)
	}
	fetchitems := func(ids []string) error {
		_, items, err := s.api.FetchWorkItemsByIDs(proj.RefID, ids)
		if err != nil {
			return fmt.Errorf("error with fetchWorkItemsByIDs, err: %v", err)
		}
		for _, i := range items {
			if err := sender.Send(i); err != nil {
				return err
			}
		}
		return nil
	}
	var ids []string
	for _, id := range allids {
		ids = append(ids, id)
		if len(ids) == 200 {
			err := fetchitems(ids)
			if err != nil {
				return err
			}
			ids = []string{}
		}
	}
	if len(ids) > 0 {
		err := fetchitems(ids)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Integration) processSprints(ctx *repoprojects.ProjectCtx, proj Project, teamids []string) error {
	sender, err := ctx.Session(work.SprintModelName)
	if err != nil {
		return err
	}
	for _, teamid := range teamids {
		sprints, err := s.api.FetchSprint(proj.RefID, teamid)
		if err != nil {
			return err
		}
		for _, sp := range sprints {
			if err := sender.Send(sp); err != nil {
				return err
			}
		}
	}
	return nil
}
