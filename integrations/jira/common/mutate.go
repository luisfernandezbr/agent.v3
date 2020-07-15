package common

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pinpt/agent/integrations/jira/commonapi"
	"github.com/pinpt/agent/integrations/pkg/mutate"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-common/v10/datamodel"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/work"
)

func (s *JiraCommon) returnUpdatedIssue(issueRefID string) (res rpcdef.MutateResult, rerr error) {
	qc := s.CommonQC()
	issue, err := commonapi.IssueByIDFieldsForMutation(qc, issueRefID)
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

func (s *JiraCommon) mutationResult(modelName datamodel.ModelNameType, obj Model) (res rpcdef.MutateResult, rerr error) {
	objs := rpcdef.MutatedObjects{}
	objs[modelName.String()] = []interface{}{obj.ToMap()}
	res.MutatedObjects = objs
	return
}

func (s *JiraCommon) Mutate(ctx context.Context, fn, data string, config rpcdef.ExportConfig) (res rpcdef.MutateResult, _ error) {

	rerr := func(err error) {
		res = mutate.ResultFromError(err)
	}

	var action agent.IntegrationMutationRequestAction
	err := action.FromInterface(fn)
	if err != nil {
		rerr(err)
		return
	}

	qc := s.CommonQC()

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
		comment, err := commonapi.AddComment(qc, obj.IssueRefID, obj.Body)
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
		err = commonapi.EditTitle(qc, obj.IssueID, obj.Title)
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
		err = commonapi.EditStatus(qc, obj.IssueID, obj.TransitionID, obj.Fields)
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
		err = commonapi.EditPriority(qc, obj.IssueID, obj.PriorityID)
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
		err = commonapi.AssignUser(qc, obj.IssueID, obj.UserID)
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
		transitions, err := commonapi.GetIssueTransitions(qc, obj.IssueID)
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
