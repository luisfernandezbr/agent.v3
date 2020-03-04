package jiracommon

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
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

type CustomFieldIDs struct {
	StoryPoints string
	Epic        string
}

func (s CustomFieldIDs) missing() (res []string) {
	if s.StoryPoints == "" {
		res = append(res, "StoryPoints")
	}
	if s.Epic == "" {
		res = append(res, "Epic")
	}
	return
}

type issueResolver struct {
	qc    jiracommonapi.QueryContext
	cache map[string]string
}

func newIssueResolver(qc jiracommonapi.QueryContext) *issueResolver {
	s := &issueResolver{}
	s.qc = qc
	s.cache = map[string]string{}
	return s
}

func (s *issueResolver) IssueRefIDFromKey(key string) (refID string, rerr error) {
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

func (s *issueResolver) issueRefIDFromKeyNoCache(key string) (refID string, rerr error) {
	refID, err := jiracommonapi.IssueRefIDFromKey(s.qc, key)
	if err != nil {
		rerr = fmt.Errorf("could not query issue: %v", err)
		return
	}
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
	issueResolver := newIssueResolver(qc)

	senderIssues, err := ctx.Session(work.IssueModelName)
	if err != nil {
		return err
	}

	customFieldIDs := CustomFieldIDs{}

	for key, val := range fieldByID {
		switch val.Name {
		case "Story Points":
			customFieldIDs.StoryPoints = key
		case "Epic Link":
			customFieldIDs.Epic = key
		}
	}

	if len(customFieldIDs.missing()) == 0 {
		logger.Debug("found all custom field ids")
	} else {
		logger.Warn("some custom field ids were not found", "missing", customFieldIDs.missing())
	}

	err = jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, rerr error) {
		pi, resIssues, err := jiracommonapi.IssuesAndChangelogsPage(qc, project.Project, fieldByID, senderIssues.LastProcessedTime(), paginationParams)
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

				if f.ID == "" || f.Value == "" {
					continue
				}

				// TODO: BUG: These fields would not be set when returning updated issues after writing to jira, or when returning updated objects from hooks
				// we can't easily move those to convertIssue, since this requires querying custom fields first
				switch f.ID {
				case customFieldIDs.StoryPoints:
					// story points can be expressed as fractions or whole numbers so convert it to a float
					sp, err := strconv.ParseFloat(f.Value, 32)
					if err != nil {
						logger.Error("error parsing Story Point value", "v", f.Value, "err", err, "key", issue.Identifier)
					} else {
						issue.StoryPoints = &sp
					}
				case customFieldIDs.Epic:
					refID, err := issueResolver.IssueRefIDFromKey(f.Value)
					if err != nil {
						logger.Error("could not convert epic key to ref id", "v", f.Value, "err", err)
						continue
					}
					epicID := qc.IssueID(refID)
					issue.EpicID = &epicID
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
			err := s.exportIssueComments(senderIssues, project, obj.RefID)
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
	issueRefID string) error {
	s.opts.Logger.Debug("exporting comments for issue", "project", project, "issue_ref_id", issueRefID)

	senderComments, err := senderIssues.Session(work.IssueCommentModelName.String(), issueRefID, issueRefID)
	if err != nil {
		return err
	}

	err = jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, rerr error) {
		pi, res, err := jiracommonapi.IssueComments(s.CommonQC(), project.Project, issueRefID, paginationParams)
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
