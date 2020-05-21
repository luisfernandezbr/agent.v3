package commonapi

import (
	"net/url"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/integration-sdk/work"
)

const IssueCommentsExpandParam = "renderedBody"

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

	params.Add("expand", IssueCommentsExpandParam)

	qc.Logger.Debug("issue comments request", "project", project.Key, "issue", issueRefID, "params", params)

	var rr struct {
		Total      int               `json:"total"`
		MaxResults int               `json:"maxResults"`
		Comments   []CommentResponse `json:"comments"`
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
		item, err := ConvertComment(qc, data, "", &IssueKeys{
			ProjectRefID: project.JiraID,
			IssueRefID:   issueRefID,
			IssueKey:     issueKey,
		})
		if err != nil {
			rerr = err
			return
		}
		resIssueComments = append(resIssueComments, item)
	}

	return
}

type CommentResponse struct {
	Self   string `json:"self"`
	ID     string `json:"id"`
	Author struct {
		Key       string `json:"key"`       // hosted,cloud
		AccountID string `json:"accountID"` // cloud only
	} `json:"author"`
	RenderedBody string `json:"renderedBody"`
	Created      string `json:"created"`
	Updated      string `json:"updated"`
}

func ConvertComment(qc QueryContext, data CommentResponse, issueIDOrKeyOptional string, issueKeysOptional *IssueKeys) (_ *work.IssueComment, rerr error) {

	var issueKeys IssueKeys

	if issueKeysOptional != nil {
		issueKeys = *issueKeysOptional
	} else {
		var err error
		issueKeys, err = GetIssueKeys(qc, issueIDOrKeyOptional)
		if err != nil {
			rerr = err
			return
		}
	}

	item := &work.IssueComment{}
	item.CustomerID = qc.CustomerID
	item.RefType = "jira"
	item.RefID = data.ID

	item.ProjectID = qc.ProjectID(issueKeys.ProjectRefID)
	item.IssueID = qc.IssueID(issueKeys.IssueRefID)

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

	item.URL = qc.IssueCommentURL(issueKeys.IssueKey, data.ID)
	return item, nil
}
