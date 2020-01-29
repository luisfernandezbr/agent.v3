package jiracommon

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/integrationid"
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

	processOpts.IntegrationType = integrationid.TypeSourcecode
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

func (s *JiraCommon) issuesAndChangelogsForProject(
	ctx *repoprojects.ProjectCtx,
	project Project,
	fieldByID map[string]jiracommonapi.CustomField,
	sprints *Sprints) error {

	s.opts.Logger.Info("processing issues and changelogs for project", "project", project.Key)

	senderIssues, err := ctx.Session(work.IssueModelName)
	if err != nil {
		return err
	}

	// find the custom_id field id for Story Points
	var storyPointCustomFieldID *string
	for key, val := range fieldByID {
		if val.Name == "Story Points" {
			storyPointCustomFieldID = &key
			break
		}
	}

	err = jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, rerr error) {
		pi, resIssues, err := jiracommonapi.IssuesAndChangelogsPage(s.CommonQC(), project.Project, fieldByID, senderIssues.LastProcessedTime(), paginationParams)
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
						s.opts.Logger.Error("could not process Sprint field value", "v", f.Value, "err", err, "key", issue.Identifier)
					}
					continue
				}
				// check and see if this custom field is a story point custom field and if so, extract
				// the current story point value.
				if storyPointCustomFieldID != nil && f.ID == *storyPointCustomFieldID {
					// story point value can be NULL indicating we didn't set it which is different
					// than a 0 value
					if f.Value != "" {
						// story points can be expressed as fractions or whole numbers so convert it to a float
						sp, err := strconv.ParseFloat(f.Value, 32)
						if err != nil {
							s.opts.Logger.Error("error parsing Story Point value", "v", f.Value, "err", err, "key", issue.Identifier)
						} else {
							issue.StoryPoints = &sp
						}
					}
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

		return pi.HasMore, pi.MaxResults, nil
	})
	if err != nil {
		return err
	}

	return nil
}
