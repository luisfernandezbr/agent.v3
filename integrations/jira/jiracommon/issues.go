package jiracommon

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/integrations/jira/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/work"
)

type Project struct {
	jiracommonapi.Project
}

func (s Project) GetID() string {
	return s.Project.JiraID
}

func (s Project) GetReadableID() string {
	return s.Project.Key
}

const issuesAndChangelogsProjectConcurrency = 10

func projectsToChan(projects []Project) chan Project {
	res := make(chan Project)
	go func() {
		for _, p := range projects {
			res <- p
		}
		close(res)
	}()
	return res
}

func (s *JiraCommon) IssuesAndChangelogs(
	projectSender *objsender.Session,
	projects []Project,
	fieldByID map[string]jiracommonapi.CustomField) (_ []rpcdef.ExportProject, rerr error) {

	projectSender.SetNoAutoProgress(true)
	projectSender.SetTotal(len(projects))

	sprints := NewSprints()

	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.opts.Logger
	processOpts.ProjectFn = func(ctx *repoprojects.ProjectCtx) error {
		project := ctx.Project.(Project)
		return s.issuesAndChangelogsForProject(ctx, project, fieldByID, sprints)
	}

	processOpts.Concurrency = issuesAndChangelogsProjectConcurrency
	var projectsIface []repoprojects.RepoProject
	for _, p := range projects {
		projectsIface = append(projectsIface, p)
	}
	processOpts.Projects = projectsIface

	processOpts.IntegrationType = inconfig.IntegrationTypeSourcecode
	processOpts.CustomerID = s.opts.CustomerID
	processOpts.RefType = "jira"
	processOpts.Sender = projectSender

	processor := repoprojects.NewProcess(processOpts)
	exportResult, err := processor.Run()
	if err != nil {
		rerr = err
		return
	}

	senderSprints, err := objsender.Root(s.agent, work.SprintModelName.String())
	if err != nil {
		rerr = err
		return
	}

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
			rerr = fmt.Errorf("invalid status for sprint: %v", data.State)
			return
		}

		err = senderSprints.Send(item)
		if err != nil {
			rerr = err
			return
		}
	}

	err = senderSprints.Done()
	if err != nil {
		rerr = err
		return
	}

	return exportResult, nil

}

type IssueResolver struct {
	qc    jiracommonapi.QueryContext
	cache map[string]string
}

func NewIssueResolver(qc jiracommonapi.QueryContext) *IssueResolver {
	s := &IssueResolver{}
	s.qc = qc
	s.cache = map[string]string{}
	return s
}

func (s *IssueResolver) IssueRefIDFromKey(key string) (refID string, rerr error) {
	if s.cache[key] != "" {
		return s.cache[key], nil
	}
	res, err := s.issueRefIDFromKeyNoCache(key)
	if err != nil {
		rerr = err
		return
	}
	s.cache[key] = res
	return res, nil
}

func (s *IssueResolver) issueRefIDFromKeyNoCache(key string) (refID string, rerr error) {
	if key == "" {
		return "", errors.New("empty refID passed to IssueRefIDFromKey")
	}
	keys, err := jiracommonapi.GetIssueKeys(s.qc, key)
	if err != nil {
		rerr = fmt.Errorf("could not query issue: %v", err)
		return
	}
	refID = keys.IssueRefID
	return refID, nil
}

func (s *JiraCommon) issuesAndChangelogsForProject(
	ctx *repoprojects.ProjectCtx,
	project Project,
	fieldByID map[string]jiracommonapi.CustomField,
	sprints *Sprints) error {

	logger := s.opts.Logger

	logger.Info("processing issues and changelogs for project", "project", project.Key)

	qc := s.CommonQC()
	issueResolver := NewIssueResolver(qc)

	senderIssues, err := ctx.Session(work.IssueModelName)
	if err != nil {
		return err
	}

	err = jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, rerr error) {
		pi, resIssues, err := jiracommonapi.IssuesAndChangelogsPage(qc, project.Project, fieldByID, senderIssues.LastProcessedTime(), paginationParams, issueResolver.IssueRefIDFromKey)
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
						logger.Error("could not process Sprint field value", "v", f.Value, "err", err, "key", issue.Identifier)
					}
					continue
				}
			}
		}

		err = senderIssues.SetTotal(pi.Total)
		if err != nil {
			rerr = err
			return
		}
		for _, obj := range resIssues {
			err := senderIssues.Send(obj)
			if err != nil {
				rerr = err
				return
			}
		}
		for _, obj := range resIssues {
			err := s.exportIssueComments(senderIssues, project, obj.RefID, obj.Identifier)
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

func (s *JiraCommon) exportIssueComments(
	senderIssues *objsender.Session,
	project Project,
	issueRefID string,
	issueKey string) error {
	s.opts.Logger.Debug("exporting comments for issue", "project", project, "issue_ref_id", issueRefID)

	senderComments, err := senderIssues.Session(work.IssueCommentModelName.String(), issueRefID, issueRefID)
	if err != nil {
		return err
	}

	err = jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, rerr error) {
		pi, res, err := jiracommonapi.IssueComments(s.CommonQC(), project.Project, issueRefID, issueKey, paginationParams)
		if err != nil {
			rerr = err
			return
		}
		for _, item := range res {
			err := senderComments.Send(item)
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

	return senderComments.Done()
}

func (s *JiraCommon) IssueTypes(sender *objsender.Session) error {
	s.opts.Logger.Debug("exporting issue types")

	res, err := jiracommonapi.IssueTypes(s.CommonQC())
	if err != nil {
		return err
	}
	for _, item := range res {
		err := sender.Send(&item)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *JiraCommon) IssuePriorities(sender *objsender.Session) error {
	s.opts.Logger.Debug("exporting issue priorities")

	res, err := jiracommonapi.Priorities(s.CommonQC())
	if err != nil {
		return err
	}
	for _, item := range res {
		err := sender.Send(&item)
		if err != nil {
			return err
		}
	}

	return nil
}
