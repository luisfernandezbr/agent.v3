package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pinpt/agent/integrations/jira-cloud/api"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-datamodel/agent"
)

func (s *Integration) Mutate(ctx context.Context, fn, data string, config rpcdef.ExportConfig) (rerr error) {
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

	switch action {
	case agent.IntegrationMutationRequestActionIssueAddComment:
		var obj struct {
			IssueRefID string `json:"ref_id"`
			Body       string `json:"body"`
		}
		err := json.Unmarshal([]byte(data), &obj)
		if err != nil {
			return err
		}
		return api.AddComment(s.qc, obj.IssueRefID, obj.Body)
	default:
		return fmt.Errorf("mutate fn not supported: %v", fn)
	}
}
