package jiracommonapi

import (
	"net/url"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/work"
)

func IssueComments(
	qc QueryContext,
	project Project,
	issueRefID string,
	issueKey string,
	paginationParams url.Values) (pi PageInfo, resIssueComments []*work.IssueComment, rerr error) {

	objectPath := "issue/" + issueRefID + "/comment"
	params := paginationParams

	//params.Set("maxResults", "1") // for testing
	params.Set("validateQuery", "strict")

	params.Add("expand", "renderedBody")

	qc.Logger.Debug("issue comments request", "project", project.Key, "issue", issueRefID, "params", params)

	var rr struct {
		Total      int `json:"total"`
		MaxResults int `json:"maxResults"`
		Comments   []struct {
			Self   string `json:"self"`
			ID     string `json:"id"`
			Author struct {
				Key       string `json:"key"`       // hosted,cloud
				AccountID string `json:"accountID"` // cloud only
			} `json:"author"`
			RenderedBody string `json:"renderedBody"`
			Created      string `json:"created"`
			Updated      string `json:"updated"`
		} `json:"comments"`
	}

	err := qc.Req.Get(objectPath, params, &rr)
	if err != nil {
		rerr = err
		return
	}

	pi.Total = rr.Total
	pi.MaxResults = rr.MaxResults
	if len(rr.Comments) == rr.MaxResults {
		pi.HasMore = true
	}

	for _, data := range rr.Comments {
		item := &work.IssueComment{}
		item.CustomerID = qc.CustomerID
		item.RefType = "jira"
		item.RefID = data.ID

		item.ProjectID = qc.ProjectID(project.JiraID)
		item.IssueID = qc.IssueID(issueRefID)

		created, err := ParseTime(data.Created)
		if err != nil {
			rerr = err
			return
		}
		date.ConvertToModel(created, &item.CreatedDate)
		updated, err := ParseTime(data.Updated)
		if err != nil {
			rerr = err
			return
		}
		date.ConvertToModel(updated, &item.UpdatedDate)

		authorID := data.Author.AccountID // cloud jira
		if authorID == "" {
			authorID = data.Author.Key // hosted jira
		}

		item.UserRefID = authorID
		item.Body = adjustRenderedHTML(qc.WebsiteURL, data.RenderedBody)

		item.URL = qc.IssueCommentURL(issueKey, data.ID)

		resIssueComments = append(resIssueComments, item)
	}

	return
}
