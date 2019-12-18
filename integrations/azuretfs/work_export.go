package main

import (
	"fmt"

	"github.com/pinpt/agent/integrations/pkg/objsender"

	azureapi "github.com/pinpt/agent/integrations/azuretfs/api"
	"github.com/pinpt/integration-sdk/work"
)

func (s *Integration) exportWork() error {

	var orgname string
	if s.Creds.Organization != nil {
		orgname = *s.Creds.Organization
	} else {
		orgname = *s.Creds.Collection
	}
	sender, err := s.orgSession.Session(work.ProjectModelName.String(), orgname, orgname)
	if err != nil {
		return err
	}
	projects, err := s.api.FetchProjects(s.ExcludedProjectIDs)
	if err != nil {
		return err
	}
	if err := sender.SetTotal(len(projects)); err != nil {
		s.logger.Error("error setting total projects on exportWork", "err", err)
	}
	for _, proj := range projects {
		if err := sender.Send(proj); err != nil {
			s.logger.Error("error sending project", "id", proj.RefID, "err", err)
			return err
		}
		teamids, err := s.api.FetchTeamIDs(proj.RefID)
		if err != nil {
			return err
		}
		if err = s.processWorkUsers(proj.RefID, proj.Name, teamids, sender); err != nil {
			return err
		}
		if err = s.processWorkItems(proj.RefID, proj.Name, sender); err != nil {
			return err
		}
		if err = s.processSprints(proj.RefID, proj.Name, teamids, sender); err != nil {
			return err
		}
	}
	return sender.Done()
}

func (s *Integration) processWorkUsers(projid, projname string, teamids []string, sender *objsender.Session) error {
	sender, err := sender.Session(work.UserModelName.String(), projid, projname)
	if err != nil {
		s.logger.Error("error creating sender session for work user")
		return err
	}
	users, err := s.api.FetchWorkUsers(projid, teamids)
	if err != nil {
		return fmt.Errorf("error fetching users. err %s", err.Error())
	}
	if err := sender.SetTotal(len(users)); err != nil {
		s.logger.Error("error setting total users on processWorkUsers", "err", err)
	}
	for _, user := range users {
		if err := sender.Send(user); err != nil {
			s.logger.Error("error sending users", "err", err, "id", user.RefID)
		}
	}
	return sender.Done()
}

func (s *Integration) processWorkItems(projid, projname string, sender *objsender.Session) error {
	sender, err := sender.Session(work.IssueModelName.String(), projid, projname)
	if err != nil {
		s.logger.Error("error creating sender session for work user")
		return err
	}

	// gets the work items (issues) and sends them to the items channel
	// The first step is to get the IDs of all items that changed after the fromdate
	// Then we need to get the items 200 at a time, this is done async
	async := azureapi.NewAsync(s.Concurrency)
	allids, err := s.api.FetchItemIDs(projid, sender.LastProcessedTime())
	if err != nil {
		return err
	}
	if err := sender.SetTotal(len(allids)); err != nil {
		s.logger.Error("error setting total ids on processWorkItems", "err", err)
	}
	fetchitems := func(ids []string) {
		async.Do(func() {
			_, items, err := s.api.FetchWorkItemsByIDs(projid, ids)
			if err != nil {
				s.logger.Error("error with fetchWorkItemsByIDs", "err", err)
				return
			}
			for _, i := range items {
				if err := sender.Send(i); err != nil {
					s.logger.Error("error sending work item", "err", err, "id", i.RefID)
				}
			}
		})
	}
	var ids []string
	for _, id := range allids {
		ids = append(ids, id)
		if len(ids) == 200 {
			fetchitems(ids)
			ids = []string{}
		}
	}
	if len(ids) > 0 {
		fetchitems(ids)
	}
	async.Wait()
	return sender.Done()
}

func (s *Integration) processSprints(projid, projname string, teamids []string, sender *objsender.Session) error {
	sender, err := sender.Session(work.SprintModelName.String(), projid, projname)
	if err != nil {
		s.logger.Error("error creating sender session for work sprint")
		return err
	}
	async := azureapi.NewAsync(s.Concurrency)
	for _, teamid := range teamids {
		teamid := teamid
		async.Do(func() {
			sprints, err := s.api.FetchSprint(projid, teamid)
			if err != nil {
				return
			}
			for _, sp := range sprints {
				if err := sender.Send(sp); err != nil {
					s.logger.Error("error sending sprint", "err", err, "id", sp.RefID)
				}
			}
		})
	}
	async.Wait()
	return sender.Done()
}
