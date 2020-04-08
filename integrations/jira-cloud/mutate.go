package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pinpt/agent/integrations/jira-cloud/api"
	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/mutate"
	"github.com/pinpt/agent/pkg/requests2"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/work"
	"golang.org/x/exp/errors"
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

func (s *Integration) returnUpdatedIssue(issueRefID string) (res rpcdef.MutateResult, rerr error) {
	issue, err := jiracommonapi.IssueByID(s.qc.Common(), issueRefID)
	if err != nil {
		rerr = err
		return
	}
	m := issue.ToMap()
	delete(m, "planned_start_date")
	delete(m, "planned_end_date")
	delete(m, "epic_id")
	delete(m, "story_points")
	objs := rpcdef.MutatedObjects{}
	objs[work.IssueModelName.String()] = []interface{}{m}
	res.MutatedObjects = objs
	return
}

type Model interface {
	ToMap() map[string]interface{}
}

func (s *Integration) mutationResult(modelName datamodel.ModelNameType, obj Model) (res rpcdef.MutateResult, rerr error) {
	objs := rpcdef.MutatedObjects{}
	objs[modelName.String()] = []interface{}{obj.ToMap()}
	res.MutatedObjects = objs
	return
}

func (s *Integration) Mutate(ctx context.Context, fn, data string, config rpcdef.ExportConfig) (res rpcdef.MutateResult, _ error) {

	rerr := func(err error) {
		var e requests2.StatusCodeError
		if errors.As(err, &e) && e.Got == http.StatusNotFound {
			res.ErrorCode = mutate.ErrNotFound
		}
		res.Error = err.Error()
	}

	err := s.initWithConfig(config, false)
	if err != nil {
		rerr(err)
		return
	}

	action := unmarshalAction(fn)

	err = action.UnmarshalJSON([]byte(fn))
	if err != nil {
		rerr(err)
		return
	}

	var common struct {
		IssueID string `json:"ref_id"`
	}
	err = json.Unmarshal([]byte(data), &common)
	if err != nil {
		rerr(err)
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
			rerr(err)
			return
		}
		comment, err := api.AddComment(s.qc, obj.IssueRefID, obj.Body)
		if err != nil {
			rerr(err)
			return
		}
		return s.mutationResult(work.IssueCommentModelName, comment)
	case agent.IntegrationMutationRequestActionIssueSetTitle:
		var obj struct {
			IssueID string `json:"ref_id"`
			Title   string `json:"title"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr(err)
			return
		}
		err = api.EditTitle(s.qc, obj.IssueID, obj.Title)
		if err != nil {
			rerr(err)
			return
		}
		return s.returnUpdatedIssue(obj.IssueID)
	case agent.IntegrationMutationRequestActionIssueSetStatus:
		var obj struct {
			IssueID      string            `json:"ref_id"`
			TransitionID string            `json:"transition_id"`
			Fields       map[string]string `json:"fields"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr(err)
			return
		}
		err = api.EditStatus(s.qc, obj.IssueID, obj.TransitionID, obj.Fields)
		if err != nil {
			rerr(err)
			return
		}
		return s.returnUpdatedIssue(obj.IssueID)
	case agent.IntegrationMutationRequestActionIssueSetPriority:
		var obj struct {
			IssueID    string `json:"ref_id"`
			PriorityID string `json:"priority_ref_id"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr(err)
			return
		}
		err = api.EditPriority(s.qc, obj.IssueID, obj.PriorityID)
		if err != nil {
			rerr(err)
			return
		}
		return s.returnUpdatedIssue(obj.IssueID)
	case agent.IntegrationMutationRequestActionIssueSetAssignee:
		var obj struct {
			IssueID string `json:"ref_id"`
			UserID  string `json:"user_ref_id"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr(err)
			return
		}
		err = api.AssignUser(s.qc, obj.IssueID, obj.UserID)
		if err != nil {
			rerr(err)
			return
		}
		return s.returnUpdatedIssue(obj.IssueID)
	case agent.IntegrationMutationRequestActionIssueGetTransitions:
		var obj struct {
			IssueID string `json:"ref_id"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			rerr(err)
			return
		}
		transitions, err := api.GetIssueTransitions(s.qc, obj.IssueID)
		if err != nil {
			rerr(err)
			return
		}
		res.WebappResponse = transitions
		return
	}

	rerr(fmt.Errorf("mutate fn not supported: %v", fn))
	return
}
