package azurecommon

import (
	"fmt"

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
	return nil
}

func (s *Integration) processProjects() ([]string, error) {
	sender := objsender.NewNotIncremental(s.agent, work.ProjectModelName.String())
	items := make(chan work.Project)
	done := make(chan bool)
	go func() {
		s.logger.Info("started with project")
		count := 0
		for each := range items {
			s.logger.Info("project sent", each.Stringify())
			if err := sender.Send(&each); err != nil {
				s.logger.Error("error sending work.Project", "err", err)
			}
			count++
		}
		s.logger.Info("ended with project", "count", count)
		done <- true
	}()
	projids, err := s.api.FetchProjects(items)
	close(items)
	<-done
	if e := sender.Done(); e != nil {
		s.logger.Error("error calling sender.Done() in s.processProjects()", "err", e)
	}
	return projids, err
}

func (s *Integration) processWorkUsers(usrs []string) error {
	sender := objsender.NewNotIncremental(s.agent, work.UserModelName.String())
	items := make(chan work.User)
	done := make(chan bool)
	go func() {
		s.logger.Info("started with users")
		count := 0
		for each := range items {
			if err := sender.Send(&each); err != nil {
				s.logger.Error("error sending work.User", "err", err)
			}
			count++
		}
		s.logger.Info("ended with users", "count", count)
		done <- true
	}()

	err := s.api.FetchWorkUsers(usrs, items)
	close(items)
	<-done
	if e := sender.Done(); e != nil {
		s.logger.Error("error calling sender.Done() in s.processWorkUsers()", "err", e)
	}
	return err
}

func (s *Integration) processWorkItems(projids []string) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, work.IssueModelName.String())
	if err != nil {
		return err
	}
	items := make(chan azureapi.WorkItemsResult)
	done := make(chan bool)
	go func() {
		for result := range items {
			count := 0
			s.logger.Info("started processing work items, project id" + result.ProjectID)
			for each := range result.Issues {
				if err := sender.Send(&each); err != nil {
					s.logger.Error("error sending work.Issue", "err", err)
				}
				count++
				if (count % 1000) == 0 {
					s.logger.Info(fmt.Sprintf("%d", count) + " issues sent")
				}
			}
			s.logger.Info("ended processing work items, project id "+result.ProjectID, "count", count)
		}
		done <- true
	}()
	err = s.api.FetchWorkItems(projids, sender.LastProcessed, items)
	close(items)
	<-done
	if e := sender.Done(); e != nil {
		s.logger.Error("error calling sender.Done() in s.processWorkItems()", "err", e)
	}
	return err
}

func (s *Integration) processChangelogs(projids []string) error {
	s.logger.Info("=========")
	sender := objsender.NewNotIncremental(s.agent, work.ChangelogModelName.String())
	items := make(chan azureapi.WorkChangelogsResult)
	done := make(chan bool)
	go func() {
		for result := range items {
			count := 0
			s.logger.Info("changelog - started with project " + result.ProjectID)
			for each := range result.Changelogs {
				if err := sender.Send(&each); err != nil {
					s.logger.Error("changelog - error sending work.Issue", "err", err)
				}
				count++
				if (count % 1000) == 0 {
					s.logger.Info(fmt.Sprintf("%d", count) + " changelogs sent")
				}
			}
			s.logger.Info("changelog - ended with project "+result.ProjectID, "count", count)
		}
		done <- true
	}()
	err := s.api.FetchChangelogs(projids, items)
	close(items)
	<-done
	if e := sender.Done(); e != nil {
		s.logger.Error("changelog - error calling sender.Done() in s.processChangelogs()", "err", e)
	}
	return err

}
