package api

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/pkg/requests2"
	"github.com/pinpt/integration-sdk/work"
)

// AddComment adds a comment to issueID
// currently adding body as simple unformatted text
// to support formatting need to use Atlassian Document Format
// haven't found a way to pass text with atlassian tags, such as {code}
// https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
func AddComment(qc QueryContext, issueID, body string) (_ *work.IssueComment, rerr error) {
	qc.Logger.Info("adding comment", "issue", issueID, "body", body)

	content := []map[string]interface{}{
		{
			"type": "paragraph",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": body,
				},
			},
		},
	}

	type Body struct {
		Type    string      `json:"type"`
		Version int         `json:"version"`
		Content interface{} `json:"content"`
	}

	reqObj := struct {
		Body Body `json:"body"`
	}{
		Body: Body{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}

	params := url.Values{}
	params.Add("expand", jiracommonapi.IssueCommentsExpandParam)

	var res jiracommonapi.CommentResponse

	req := requests2.Request{}
	req.Method = "POST"
	req.URL = qc.Req.URL("issue/" + issueID + "/comment")
	req.Query = params
	var err error
	req.Body, err = json.Marshal(reqObj)
	if err != nil {
		rerr = err
		return
	}
	_, err = qc.Req.JSON(req, &res)
	if err != nil {
		rerr = err
		return
	}

	return jiracommonapi.ConvertComment(qc.Common(), res, issueID, nil)
}

func mutJSONReq(qc QueryContext, method string, uri string, body interface{}, res interface{}) error {
	req := requests2.Request{}
	req.Method = method
	req.URL = qc.Req.URL(uri)
	var err error
	req.Body, err = json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = qc.Req.JSON(req, &res)
	if err != nil {
		return err
	}
	return nil
}

func EditTitle(qc QueryContext, issueID, title string) error {
	qc.Logger.Info("editing issue title", "issue", issueID, "title", title)

	reqObj := struct {
		Fields struct {
			Summary string `json:"summary"`
		} `json:"fields"`
	}{}
	reqObj.Fields.Summary = title
	return mutJSONReq(qc, "PUT", "issue/"+issueID, reqObj, nil)
}

func EditPriority(qc QueryContext, issueID, priorityID string) error {
	qc.Logger.Info("editing issue priority", "issue", issueID, "priority_id", priorityID)

	reqObj := struct {
		Fields struct {
			Priority struct {
				ID string `json:"id"`
			} `json:"priority"`
		} `json:"fields"`
	}{}
	reqObj.Fields.Priority.ID = priorityID
	return mutJSONReq(qc, "PUT", "issue/"+issueID, reqObj, nil)
}

type issueTransition struct {
	ID string `json:"id"`
	To struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"to"`
}

func getTransitions(qc QueryContext, issueID string) (res []issueTransition, rerr error) {
	var obj struct {
		Transitions []issueTransition `json:"transitions"`
	}
	err := mutJSONReq(qc, "GET", "issue/"+issueID+"/transitions", nil, &obj)
	if err != nil {
		rerr = err
		return
	}
	return obj.Transitions, err
}

// EditStatus changes issue status to passed id.
func EditStatus(qc QueryContext, issueID, statusID string) error {
	qc.Logger.Info("editing issue status", "issue", issueID, "statusID", statusID)

	// get transition ids first
	transitions, err := getTransitions(qc, issueID)
	if err != nil {
		return err
	}
	var transition issueTransition
	for _, tr := range transitions {
		if tr.To.ID == statusID {
			transition = tr
			break
		}
	}
	if transition.ID == "" {
		statuses := []string{}
		for _, tr := range transitions {
			statuses = append(statuses, fmt.Sprintf("%+v", tr.To))
		}
		return fmt.Errorf("could not change issue status: invalid status id: %v options: %v", statusID, statuses)
	}

	reqObj := struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
	}{}
	reqObj.Transition.ID = transition.ID

	return mutJSONReq(qc, "POST", "issue/"+issueID+"/transitions", reqObj, nil)
}

func AssignUser(qc QueryContext, issueID, accountID string) error {
	qc.Logger.Info("change issue assignee", "issue", issueID, "account_id", accountID)

	reqObj := struct {
		AccountID string `json:"accountId,omitempty"`
	}{}
	reqObj.AccountID = accountID

	return mutJSONReq(qc, "PUT", "issue/"+issueID+"/assignee", reqObj, nil)
}
