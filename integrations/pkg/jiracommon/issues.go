package jiracommon

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/go-datamodel/work"
)

type Project = jiracommonapi.Project

func (s *JiraCommon) IssuesAndChangelogs(projects []Project, fieldByID map[string]*work.CustomField) error {
	senderIssues, err := objsender.NewIncrementalDateBased(s.agent, "work.issue")
	if err != nil {
		return err
	}
	defer senderIssues.Done()

	senderChangelogs := objsender.NewNotIncremental(s.agent, "work.changelog")
	defer senderChangelogs.Done()

	startedSprintExport := time.Now()
	sprints := NewSprints()

	for _, p := range projects {
		err := s.issuesAndChangelogsForProject(p, fieldByID, senderIssues, senderChangelogs, sprints)
		if err != nil {
			return err
		}
	}

	senderSprints := objsender.NewNotIncremental(s.agent, "work.sprints")
	defer senderSprints.Done()

	var sprintModels []objsender.Model
	for _, data := range sprints.data {
		item := &work.Sprint{}
		item.CustomerID = s.opts.CustomerID
		item.RefType = "jira"
		item.RefID = strconv.Itoa(data.ID)

		item.Goal = data.Goal
		item.Name = data.Name

		startDate, err := jiracommonapi.ParseTime(data.StartDate)
		if err != nil {
			return fmt.Errorf("could not parse startdate for sprint: %v err: %v", data.Name, err)
		}
		date.ConvertToModel(startDate, &item.Started)

		endDate, err := jiracommonapi.ParseTime(data.EndDate)
		if err != nil {
			return fmt.Errorf("could not parse enddata for sprint: %v err: %v", data.Name, err)
		}
		date.ConvertToModel(endDate, &item.Ended)

		completeDate, err := jiracommonapi.ParseTime(data.CompleteDate)
		if err != nil {
			return fmt.Errorf("could not parse completed for sprint: %v err: %v", data.Name, err)
		}
		date.ConvertToModel(completeDate, &item.Completed)

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

		date.ConvertToModel(startedSprintExport, &item.Fetched)

		sprintModels = append(sprintModels, item)
	}
	return senderSprints.Send(sprintModels)
}

func (s *JiraCommon) issuesAndChangelogsForProject(
	project Project,
	fieldByID map[string]*work.CustomField,
	senderIssues *objsender.IncrementalDateBased,
	senderChangelogs *objsender.NotIncremental,
	sprints *Sprints) error {

	err := jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, _ error) {
		pi, resIssues, resChangelogs, err := jiracommonapi.IssuesAndChangelogsPage(s.CommonQC(), project, fieldByID, senderIssues.LastProcessed, paginationParams)
		if err != nil {
			return false, 0, err
		}
		for _, issue := range resIssues {
			for _, f := range issue.CustomFields {

				if f.Name == "Sprint" {
					if f.Value == "" {
						continue
					}
					err := sprints.processIssueSprint(issue.RefID, f.Value)
					if err != nil {
						return false, 0, err
					}

					break
				}

			}
		}

		var resIssues2 []objsender.Model
		for _, obj := range resIssues {
			resIssues2 = append(resIssues2, obj)
		}
		err = senderIssues.Send(resIssues2)
		if err != nil {
			return false, 0, err
		}

		var resChangelogs2 []objsender.Model
		for _, obj := range resChangelogs {
			resChangelogs2 = append(resChangelogs2, obj)
		}
		err = senderChangelogs.Send(resChangelogs2)
		if err != nil {
			return false, 0, err
		}

		return pi.HasMore, pi.MaxResults, nil
	})
	if err != nil {
		return err
	}

	return nil
}
