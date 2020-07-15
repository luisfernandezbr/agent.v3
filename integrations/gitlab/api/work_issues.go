package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/date"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/work"
)

func WorkIssuesPage(qc QueryContext, projectID string, usermap UsernameMap, commentChan chan []work.IssueComment, params url.Values) (pi PageInfo, res []*work.Issue, err error) {

	qc.Logger.Debug("work issues", "project", projectID)

	objectPath := pstrings.JoinURL("projects", url.QueryEscape(projectID), "issues")

	var rawissues []IssueModel

	params.Set("scope", "all")
	pi, err = qc.Request(objectPath, params, &rawissues)
	if err != nil {
		return
	}
	for _, rawissue := range rawissues {

		idparts := strings.Split(projectID, "/")
		var identifier string
		if len(idparts) == 1 {
			identifier = idparts[0] + "-" + fmt.Sprint(rawissue.Iid)
		} else {
			identifier = idparts[1] + "-" + fmt.Sprint(rawissue.Iid)
		}
		item := &work.Issue{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = fmt.Sprint(rawissue.Iid)

		item.AssigneeRefID = fmt.Sprint(rawissue.Assignee.ID)
		item.ReporterRefID = fmt.Sprint(rawissue.Author.ID)
		item.CreatorRefID = fmt.Sprint(rawissue.Author.ID)
		item.Description = rawissue.Description
		if rawissue.EpicIid != 0 {
			item.EpicID = pstrings.Pointer(fmt.Sprint(rawissue.EpicIid))
		}
		item.Identifier = identifier
		item.ProjectID = qc.IDs.WorkProject(fmt.Sprint(rawissue.ProjectID))
		item.Title = rawissue.Title
		item.Status = rawissue.State
		item.Tags = rawissue.Labels
		item.Type = "Issue"
		item.URL = rawissue.WebURL

		date.ConvertToModel(rawissue.CreatedAt, &item.CreatedDate)
		date.ConvertToModel(rawissue.UpdatedAt, &item.UpdatedDate)

		item.SprintIds = []string{qc.IDs.WorkSprintID(fmt.Sprint(rawissue.Milestone.Iid))}
		duedate, err := time.Parse("2006-01-02", rawissue.Milestone.DueDate)
		if err != nil {
			duedate = time.Time{}
		}
		date.ConvertToModel(duedate, &item.PlannedEndDate)

		startdate, err := time.Parse("2006-01-02", rawissue.Milestone.StartDate)
		if err != nil {
			startdate = time.Time{}
		}
		date.ConvertToModel(startdate, &item.PlannedStartDate)
		err = PaginateStartAt(qc.Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
			pi, changelogs, comments, err := WorkIssuesDiscussionsPage(qc, projectID, fmt.Sprint(rawissue.Iid), usermap, paginationParams)
			if err != nil {
				return page, err
			}
			item.ChangeLog = append(item.ChangeLog, changelogs...)
			commentChan <- comments
			return pi, nil
		})

		res = append(res, item)
	}

	return
}
