package jiracommon

import (
	"fmt"
	"net/url"
	"strconv"
	"sync"

	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/work"
)

type Project = jiracommonapi.Project

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
	fieldByID map[string]*work.CustomField) error {

	sprints := NewSprints()

	projectsChan := projectsToChan(projects)
	wg := sync.WaitGroup{}
	var pErr error
	var errMu sync.Mutex

	rerr := func(err error) {
		errMu.Lock()
		pErr = err
		errMu.Unlock()
	}

	for i := 0; i < issuesAndChangelogsProjectConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range projectsChan {
				errMu.Lock()
				err := pErr
				errMu.Unlock()
				if err != nil {
					return
				}
				senderIssues, err := projectSender.Session(work.IssueModelName.String(), p.JiraID, p.Key)
				if err != nil {
					rerr(err)
					return
				}

				// p is defined above
				// fieldByID is read-only
				// senderIssues and senderChangelogs are sender which support concurrency
				// sprints support concurrency for processIssueSprint
				err = s.issuesAndChangelogsForProject(p, fieldByID, senderIssues, sprints)
				if err != nil {
					rerr(err)
					return
				}

				err = senderIssues.Done()
				if err != nil {
					rerr(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if pErr != nil {
		return pErr
	}

	senderSprints, err := objsender.Root(s.agent, work.SprintModelName.String())
	if err != nil {
		return err
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
			return fmt.Errorf("invalid status for sprint: %v", data.State)
		}

		err = senderSprints.Send(item)
		if err != nil {
			return err
		}
	}

	return senderSprints.Done()
}

func (s *JiraCommon) issuesAndChangelogsForProject(
	project Project,
	fieldByID map[string]*work.CustomField,
	senderIssues *objsender.Session,
	sprints *Sprints) error {

	s.opts.Logger.Info("processing issues and changelogs for project", "project", project.Key)

	// find the custom_id field id for Story Points
	var storyPointCustomFieldID *string
	for key, val := range fieldByID {
		if val.Name == "Story Points" {
			storyPointCustomFieldID = &key
			break
		}
	}

	err := jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, rerr error) {
		pi, resIssues, err := jiracommonapi.IssuesAndChangelogsPage(s.CommonQC(), project, fieldByID, senderIssues.LastProcessedTime(), paginationParams)
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
