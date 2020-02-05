package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pinpt/agent/integrations/jira-cloud/api"
	"github.com/pinpt/agent/rpcdef"
)

func (s *Integration) Mutate(ctx context.Context, fn, data string, config rpcdef.ExportConfig) (rerr error) {
	err := s.initWithConfig(config, false)
	if err != nil {
		rerr = err
		return
	}

	switch fn {
	case "issue_add_comment":
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
