package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pinpt/agent/integrations/jira-cloud/api"
	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/mutations"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/work"
)

func unmarshalAction(fn string) (v agent.IntegrationMutationRequestAction) {
	/*
		This doesn't work due to bug in schemagen
		var action agent.IntegrationMutationRequestAction
		err = action.UnmarshalJSON([]byte("ISSUE_SET_TITLE"))
		if err != nil {
			panic(err)
		}
		//fmt.Println(action)
	*/
	switch fn {
	case "ISSUE_ADD_COMMENT":
		v = 0
	case "ISSUE_SET_TITLE":
		v = 1
	case "ISSUE_SET_STATUS":
		v = 2
	case "ISSUE_SET_PRIORITY":
		v = 3
	case "ISSUE_SET_ASSIGNEE":
		v = 4
	default:
		panic("unsupported action: " + fn)
	}
	return
}

func (s *Integration) getIssueAndReturnFields(issueRefID string, fields ...string) (_ rpcdef.MutatedObjects, rerr error) {
	res := rpcdef.MutatedObjects{}
	issue, err := jiracommonapi.IssueByID(s.qc.Common(), issueRefID)
	if err != nil {
		rerr = err
		return
	}
	issueM, err := mutations.PluckFields(issue, fields...)
	if err != nil {
		rerr = err
		return
	}
	res[work.IssueModelName.String()] = []interface{}{issueM}
	return res, nil
}

func (s *Integration) Mutate(ctx context.Context, fn, data string, config rpcdef.ExportConfig) (_ rpcdef.MutatedObjects, rerr error) {

	err := s.initWithConfig(config, false)
	if err != nil {
		rerr = err
		return
	}

	action := unmarshalAction(fn)

	s.logger.Info("action received", "a", action, "fn", fn)

	err = action.UnmarshalJSON([]byte(fn))
	if err != nil {
		rerr = err
		return
	}

	var common struct {
		IssueID string `json:"ref_id"`
	}
	err = json.Unmarshal([]byte(data), &common)
	if err != nil {
		rerr = err
		return
	}

	switch action {
	case agent.IntegrationMutationRequestActionIssueAddComment:
		rerr = errors.New("not implemented")
		return
		var obj struct {
			IssueRefID string `json:"ref_id"`
			Body       string `json:"body"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr = err
			return
		}
		err = api.AddComment(s.qc, obj.IssueRefID, obj.Body)
		if err != nil {
			rerr = err
			return
		}
	case agent.IntegrationMutationRequestActionIssueSetTitle:
		var obj struct {
			IssueID string `json:"ref_id"`
			Title   string `json:"title"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr = err
			return
		}
		err = api.EditTitle(s.qc, obj.IssueID, obj.Title)
		if err != nil {
			rerr = err
			return
		}
		res := rpcdef.MutatedObjects{}
		issue := &work.Issue{}
		issue.RefType = "jira"
		issue.RefID = obj.IssueID
		issue.Title = obj.Title
		issueM, err := mutations.PluckFields(issue, "title")
		if err != nil {
			rerr = err
			return
		}
		res[work.IssueModelName.String()] = []interface{}{issueM}
		return res, nil
	case agent.IntegrationMutationRequestActionIssueSetStatus:
		var obj struct {
			IssueID  string `json:"ref_id"`
			StatusID string `json:"status_ref_id"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr = err
			return
		}
		err = api.EditStatus(s.qc, obj.IssueID, obj.StatusID)
		if err != nil {
			rerr = err
			return
		}
		return s.getIssueAndReturnFields(obj.IssueID, "status")
	case agent.IntegrationMutationRequestActionIssueSetPriority:
		var obj struct {
			IssueID    string `json:"ref_id"`
			PriorityID string `json:"priority_ref_id"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr = err
			return
		}
		err = api.EditPriority(s.qc, obj.IssueID, obj.PriorityID)
		if err != nil {
			rerr = err
			return
		}
		return s.getIssueAndReturnFields(obj.IssueID, "priority", "priority_id")
	case agent.IntegrationMutationRequestActionIssueSetAssignee:
		var obj struct {
			IssueID string `json:"ref_id"`
			UserID  string `json:"user_ref_id"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr = err
			return
		}
		err = api.AssignUser(s.qc, obj.IssueID, obj.UserID)
		if err != nil {
			rerr = err
			return
		}
		res := rpcdef.MutatedObjects{}
		issue := &work.Issue{}
		issue.RefType = "jira"
		issue.RefID = obj.IssueID
		issue.AssigneeRefID = obj.UserID
		issueM, err := mutations.PluckFields(issue, "assignee_ref_id")
		if err != nil {
			rerr = err
			return
		}
		res[work.IssueModelName.String()] = []interface{}{issueM}
		return res, nil
	}

	rerr = fmt.Errorf("mutate fn not supported: %v", fn)
	return
}
