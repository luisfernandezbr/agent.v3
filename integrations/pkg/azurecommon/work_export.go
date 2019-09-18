package azurecommon

import (
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
	return nil
}

func (s *Integration) processProjects() (projids []string, err error) {
	sender := objsender.NewNotIncremental(s.agent, work.ProjectModelName.String())
	defer sender.Done()
	items, done := s.execute("projects", sender)
	projids, err = s.api.FetchProjects(items)
	close(items)
	<-done
	return
}

func (s *Integration) processWorkUsers(usrs []string) error {
	sender := objsender.NewNotIncremental(s.agent, work.UserModelName.String())
	defer sender.Done()
	items, done := s.execute("work users", sender)
	err := s.api.FetchWorkUsers(usrs, items)
	close(items)
	<-done
	return err
}

func (s *Integration) processWorkItems(projids []string) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, work.IssueModelName.String())
	if err != nil {
		return err
	}
	defer sender.Done()
	for _, projid := range projids {
		items, done := s.execute("work items", sender)
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
		items, done := s.execute("changelogs", sender)
		err = s.api.FetchChangelogs(projid, sender.LastProcessed, items)
		close(items)
		<-done
		if err != nil {
			return err
		}
	}
	return err

}
