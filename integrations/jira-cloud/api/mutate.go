package api

import (
	"encoding/json"

	"github.com/pinpt/agent/pkg/requests2"
)

// AddComment adds a comment to issueID
// currently adding body as simple unformatted text
// to support formatting need to use Atlassian Document Format
// haven't found a way to pass text with atlassian tags, such as {code}
// https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
func AddComment(qc QueryContext, issueID, body string) (rerr error) {
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

	req := requests2.Request{}
	req.Method = "POST"
	req.URL = qc.Req.URL("issue/" + issueID + "/comment")
	var err error
	req.Body, err = json.Marshal(reqObj)
	if err != nil {
		rerr = err
		return
	}

	var res struct {
		ID string `json:"id"`
	}

	_, err = qc.Req.JSON(req, &res)
	if err != nil {
		rerr = err
		return
	}

	return nil
}
