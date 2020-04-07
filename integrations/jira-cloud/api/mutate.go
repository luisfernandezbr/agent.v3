package api

import (
	"encoding/json"
	"net/url"

	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/mutate"
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
	Fields map[string]Field `json:"fields"`
}

type Field struct {
	Key           string         `json:"key"`
	Name          string         `json:"name"`
	Required      bool           `json:"required"`
	AllowedValues []AllowedValue `json:"allowedValues"`
}

type AllowedValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func getIssueTransitions(qc QueryContext, issueID string) (res []issueTransition, rerr error) {
	var obj struct {
		Transitions []issueTransition `json:"transitions"`
	}

	params := url.Values{}
	params.Add("expand", "transitions.fields")

	req := requests2.Request{}
	req.Method = "GET"
	req.URL = qc.Req.URL("issue/" + issueID + "/transitions")
	req.Query = params
	_, err := qc.Req.JSON(req, &obj)
	if err != nil {
		rerr = err
		return
	}
	return obj.Transitions, err
}

func GetIssueTransitions(qc QueryContext, issueID string) (res []mutate.IssueTransition, rerr error) {
	res0, err := getIssueTransitions(qc, issueID)
	if err != nil {
		rerr = err
		return
	}
	for _, iss0 := range res0 {
		iss := mutate.IssueTransition{}
		iss.ID = iss0.ID
		iss.Name = iss0.To.Name
		for _, f0 := range iss0.Fields {
			if f0.Key != "resolution" {
				if f0.Required {
					qc.Logger.Warn("transition has a required field that is not resolution, we don't support that yet, transition will happen anyway, but field will not by filled", "k", f0.Key, "n", f0.Name)
				}
				continue
			}
			f := mutate.IssueTransitionField{}
			f.ID = f0.Key
			f.Name = f0.Name
			f.Required = f0.Required
			for _, av0 := range f0.AllowedValues {
				av := mutate.AllowedValue{}
				av.ID = av0.Name // jira uses name when setting value, not id, pass name as id here
				av.Name = av0.Name
				f.AllowedValues = append(f.AllowedValues, av)
			}
			iss.Fields = append(iss.Fields, f)
		}
		res = append(res, iss)
	}
	return
}

/*
// EditStatus changes issue status to passed id.
func EditStatus(qc QueryContext, issueID, statusID string) error {
	qc.Logger.Info("editing issue status", "issue", issueID, "statusID", statusID)

	// get transition ids first
	transitions, err := getIssueTransitions(qc, issueID)
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
}*/

type transitionFieldValue struct {
	Name string `json:"name"`
}

func EditStatus(qc QueryContext, issueID, transitionID string, fieldValues map[string]string) error {
	qc.Logger.Info("editing issue status", "issue", issueID)

	reqObj := struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
		Fields map[string]transitionFieldValue `json:"fields"`
	}{}
	reqObj.Transition.ID = transitionID
	m := map[string]transitionFieldValue{}
	for k, v := range fieldValues {
		m[k] = transitionFieldValue{Name: v}
	}
	reqObj.Fields = m
	qc.Logger.Info("seting obj", "v", reqObj)
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
