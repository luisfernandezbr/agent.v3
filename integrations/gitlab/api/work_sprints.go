package api

import (
	"fmt"
	"net/url"
	"time"

	"github.com/pinpt/agent/pkg/date"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/go-datamodel/work"
)

func WorkSprintPage(qc QueryContext, projectID string, params url.Values) (pi PageInfo, res []*work.Sprint, err error) {

	qc.Logger.Debug("work sprints", "project", projectID)

	objectPath := pstrings.JoinURL("projects", url.QueryEscape(projectID), "milestones")
	var rawsprints []IssueMilestone
	pi, err = qc.Request(objectPath, params, &rawsprints)
	if err != nil {
		return
	}
	for _, rawsprint := range rawsprints {

		item := &work.Sprint{}
		item.CustomerID = qc.CustomerID
		item.RefType = qc.RefType
		item.RefID = fmt.Sprint(rawsprint.Iid)

		start, err := time.Parse("2006-01-02", rawsprint.StartDate)
		if err == nil {
			date.ConvertToModel(start, &item.StartedDate)
		} else {
			if rawsprint.StartDate != "" {
				qc.Logger.Error("could not figure out start date, skipping sprint object", "err", err, "start_date", rawsprint.StartDate)
				continue
			}
		}
		end, err := time.Parse("2006-01-02", rawsprint.DueDate)
		if err == nil {
			date.ConvertToModel(end, &item.EndedDate)
		} else {
			if rawsprint.DueDate != "" {
				qc.Logger.Error("could not figure out due date, skipping sprint object", "err", err, "due_date", rawsprint.DueDate)
				continue
			}
		}

		if rawsprint.State == "closed" {
			date.ConvertToModel(rawsprint.UpdatedAt, &item.CompletedDate)
			item.Status = work.SprintStatusClosed
		} else {
			if !start.IsZero() && start.After(time.Now()) {
				item.Status = work.SprintStatusFuture
			} else {
				item.Status = work.SprintStatusActive
			}
		}
		item.Goal = rawsprint.Description
		item.Name = rawsprint.Title
		item.RefID = fmt.Sprint(rawsprint.ID)
		item.RefType = "gitlab"

		res = append(res, item)
	}

	return
}
