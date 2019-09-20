package azurecommon

import (
	"fmt"
	"strings"

	"github.com/pinpt/agent.next/integrations/pkg/azureapi"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/work"
)

func (s *Integration) exportWork() error {
	var err error
	var projids []string
	projids, err = s.processProjects()
	if err != nil {
		return err
	}

	if err = s.processWorkUsers(projids); err != nil {
		return err
	}
	if err = s.processWorkItems(projids); err != nil {
		return err
	}
	if err = s.processChangelogs(projids); err != nil {
		return err
	}
	if err = s.processSprints(projids); err != nil {
		return err
	}
	return nil
}

func (s *Integration) processProjects() (projids []string, err error) {
	sender := objsender.NewNotIncremental(s.agent, work.ProjectModelName.String())
	defer sender.Done()
	items, done := azureapi.AsyncProcess("projects", s.logger, sender, nil)
	projids, err = s.api.FetchProjects(items)
	close(items)
	<-done
	return
}

func (s *Integration) processWorkUsers(projids []string) error {
	sender := objsender.NewNotIncremental(s.agent, work.UserModelName.String())
	defer sender.Done()
	items, done := azureapi.AsyncProcess("work users", s.logger, sender, nil)
	var errors []string
	for _, projid := range projids {
		teamids, err := s.api.FetchTeamIDs(projid)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		err = s.api.FetchWorkUsers(projid, teamids, items)
		if err != nil {
			errors = append(errors, err.Error())
		}
	}
	close(items)
	<-done
	if errors != nil {
		return fmt.Errorf("error fetching users. err %s", strings.Join(errors, ", "))
	}
	return nil
}

func (s *Integration) processWorkItems(projids []string) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, work.IssueModelName.String())
	if err != nil {
		return err
	}
	defer sender.Done()
	for _, projid := range projids {
		items, done := azureapi.AsyncProcess("work items", s.logger, sender, nil)
		err = s.api.FetchWorkItems(projid, sender.LastProcessed, items)
		close(items)
		<-done
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Integration) processChangelogs(projids []string) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, work.ChangelogModelName.String())
	if err != nil {
		return err
	}
	defer sender.Done()
	for _, projid := range projids {
		items, done := azureapi.AsyncProcess("changelogs", s.logger, sender, nil)
		err = s.api.FetchChangelogs(projid, sender.LastProcessed, items)
		close(items)
		<-done
		if err != nil {
			return err
		}
	}
	return err

}

func (s *Integration) processSprints(projids []string) error {
	sender := objsender.NewNotIncremental(s.agent, work.SprintModelName.String())
	defer sender.Done()
	for _, projid := range projids {
		items, done := azureapi.AsyncProcess("sprints", s.logger, sender, nil)
		err := s.api.FetchSprints(projid, items)
		close(items)
		<-done
		if err != nil {
			return err
		}
	}
	return nil
}
