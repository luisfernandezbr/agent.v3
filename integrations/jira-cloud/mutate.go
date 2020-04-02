package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pinpt/agent/integrations/jira-cloud/api"
	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/work"
)

func (s *Integration) Mutate(ctx context.Context, fn, data string, config rpcdef.ExportConfig) (_ rpcdef.MutatedObjects, rerr error) {
	err := s.initWithConfig(config, false)
	if err != nil {
		rerr = err
		return
	}

	var action agent.IntegrationMutationRequestAction
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
	default:
		rerr = fmt.Errorf("mutate fn not supported: %v", fn)
		return
	}

	const returnObjects = true
	if !returnObjects {
		return
	}
	res := rpcdef.MutatedObjects{}

	issue, err := jiracommonapi.IssueByID(s.qc.Common(), common.IssueID)
	if err != nil {
		rerr = err
		return
	}

	res[work.IssueModelName.String()] = []interface{}{issue.ToMap()}
	return res, nil
}
