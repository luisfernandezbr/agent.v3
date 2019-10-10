package main

import (
	"fmt"

	"github.com/pinpt/agent.next/integrations/pkg/objsender2"

	azureapi "github.com/pinpt/agent.next/integrations/azuretfs/api"
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
	projects, err := s.api.FetchProjects()
	if err != nil {
		return err
	}
	sender.SetTotal(len(projects))
	for _, proj := range projects {
		sender.Send(proj)
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

func (s *Integration) processWorkUsers(projid, projname string, teamids []string, sender *objsender2.Session) error {
	sender, err := sender.Session(work.UserModelName.String(), projid, projname)
	if err != nil {
		s.logger.Error("error creating sender session for work user")
		return err
	}
	users, err := s.api.FetchWorkUsers(projid, teamids)
	if err != nil {
		return fmt.Errorf("error fetching users. err %s", err.Error())
	}
	sender.SetTotal(len(users))
	for _, user := range users {
		sender.Send(user)
	}
	return sender.Done()
}

func (s *Integration) processWorkItems(projid, projname string, sender *objsender2.Session) error {
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
	sender.SetTotal(len(allids))
	fetchitems := func(ids []string) {
		async.Do(func() {
			_, items, err := s.api.FetchWorkItemsByIDs(projid, ids)
			if err != nil {
				s.logger.Error("error with fetchWorkItemsByIDs", "err", err)
				return
			}
			for _, i := range items {
				sender.Send(i)
				s.processChangelogs(projid, i.Identifier, i.RefID, sender)
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

func (s *Integration) processChangelogs(projid, identifier, itemid string, sender *objsender2.Session) error {

	// First we need to get the IDs of the items that hav changed after the fromdate
	// Then we need to get each changelog individually.
	sender, err := sender.Session(work.ChangelogModelName.String(), itemid, identifier)
	if err != nil {
		s.logger.Error("error creating sender session for work changelog")
		return err
	}
	changelogs, err := s.api.FetchChangeLog(projid, itemid)
	sender.SetTotal(len(changelogs))
	if err != nil {
		s.logger.Error("error fetching work item updates "+itemid, "err", err)
		return err
	}
	for _, c := range changelogs {
		sender.Send(c)
	}

	return sender.Done()
}

func (s *Integration) processSprints(projid, projname string, teamids []string, sender *objsender2.Session) error {
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
				sender.Send(sp)
			}
		})
	}
	async.Wait()
	return sender.Done()
}
