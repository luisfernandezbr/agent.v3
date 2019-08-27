package jiracommon

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/work"
)

type Project = jiracommonapi.Project

func (s *JiraCommon) IssuesAndChangelogs(projects []Project, fieldByID map[string]*work.CustomField) error {
	senderIssues, err := objsender.NewIncrementalDateBased(s.agent, work.IssueModelName.String())
	if err != nil {
		return err
	}
	senderChangelogs := objsender.NewNotIncremental(s.agent, work.ChangelogModelName.String())

	startedSprintExport := time.Now()
	sprints := NewSprints()

	for _, p := range projects {
		err := s.issuesAndChangelogsForProject(p, fieldByID, senderIssues, senderChangelogs, sprints)
		if err != nil {
			return err
		}
	}

	senderSprints := objsender.NewNotIncremental(s.agent, work.SprintModelName.String())

	for _, data := range sprints.SprintsWithIssues() {
		item := &work.Sprint{}
		item.CustomerID = s.opts.CustomerID
		item.RefType = "jira"
		item.RefID = strconv.Itoa(data.ID)

		item.Goal = data.Goal
		item.Name = data.Name

		date.ConvertToModel(data.StartDate, &item.StartedDate)
		date.ConvertToModel(data.EndDate, &item.EndedDate)
		date.ConvertToModel(data.CompleteDate, &item.CompletedDate)

		switch data.State {
		case "CLOSED":
			item.Status = work.SprintStatusClosed
		case "ACTIVE":
			item.Status = work.SprintStatusActive
		case "FUTURE":
			item.Status = work.SprintStatusFuture
		default:
			return fmt.Errorf("invalid status for sprint: %v", data.State)
		}

		date.ConvertToModel(startedSprintExport, &item.FetchedDate)

		err = senderSprints.Send(item)
		if err != nil {
			return err
		}
	}

	err = senderIssues.Done()
	if err != nil {
		return err
	}
	err = senderChangelogs.Done()
	if err != nil {
		return err
	}
	return senderSprints.Done()
}

func (s *JiraCommon) issuesAndChangelogsForProject(
	project Project,
	fieldByID map[string]*work.CustomField,
	senderIssues *objsender.IncrementalDateBased,
	senderChangelogs *objsender.NotIncremental,
	sprints *Sprints) error {

	s.opts.Logger.Info("processing issues and changelogs for project", "project", project.Key)

	err := jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, rerr error) {
		pi, resIssues, resChangelogs, err := jiracommonapi.IssuesAndChangelogsPage(s.CommonQC(), project, fieldByID, senderIssues.LastProcessed, paginationParams)
		if err != nil {
			rerr = err
			return
		}
		for _, issue := range resIssues {
			for _, f := range issue.CustomFields {

				if f.Name == "Sprint" {
					if f.Value == "" {
						continue
					}
					err := sprints.processIssueSprint(issue.RefID, f.Value)
					if err != nil {
						s.opts.Logger.Error("could not process Sprint field value", "v", f.Value, "err", err)
					}
					break
				}

			}
		}

		for _, obj := range resIssues {
			err := senderIssues.Send(obj)
			if err != nil {
				rerr = err
				return
			}
		}

		for _, obj := range resChangelogs {
			err := senderChangelogs.Send(obj)
			if err != nil {
				rerr = err
				return
			}
		}

		return pi.HasMore, pi.MaxResults, nil
	})
	if err != nil {
		return err
	}

	return nil
}
