package api

import (
	"net/url"

	"github.com/pinpt/agent.next/pkg/date"

	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"

	"github.com/pinpt/integration-sdk/agent"

	pstrings "github.com/pinpt/go-common/strings"
)

func ProjectsOnboard(qc QueryContext) (res []*agent.ProjectResponseProjects, rerr error) {

	objectPath := "project"

	params := url.Values{}
	params.Set("expand", "description")

	var rr []struct {
		Self        string `json:"self"`
		ID          string `json:"id"`
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"projectCategory"`
	}

	err := qc.Request(objectPath, params, &rr)
	if err != nil {
		rerr = err
		return
	}

	for _, data := range rr {
		item := &agent.ProjectResponseProjects{}
		item.RefID = data.ID
		item.RefType = "jira"

		item.Name = data.Name
		item.Identifier = data.Key
		item.Active = true
		item.URL = data.Self

		item.Description = pstrings.Pointer(data.Description)
		if data.Category.Name != "" {
			item.Category = pstrings.Pointer(data.Category.Name)
		}

		project := jiracommonapi.Project{JiraID: data.ID, Key: data.Key}

		lastIssue, totalIssues, err := jiracommonapi.GetProjectLastIssue(qc.Common(), project)
		if err != nil {
			if err == jiracommonapi.ErrPermissions {
				// this is a private project, skip setting last issue
				item.Name += " (No Permissions)"
			} else {
				rerr = err
				return
			}
		} else {
			item.LastIssue.IssueID = lastIssue.IssueID
			item.LastIssue.Identifier = lastIssue.Identifier

			date.ConvertToModel(lastIssue.CreatedDate, &item.LastIssue.CreatedDate)

			creator := lastIssue.Creator

			item.LastIssue.LastUser.UserID = creator.RefID()
			item.LastIssue.LastUser.Name = creator.Name
			item.LastIssue.LastUser.AvatarURL = creator.Avatars.Large

			item.TotalIssues = int64(totalIssues)
		}

		// we decide if project is active on backend TODO: this flag can be removed from datamodel
		item.Active = true

		res = append(res, item)

	}

	return
}
