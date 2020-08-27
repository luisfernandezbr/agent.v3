package commonapi

import (
	ps "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/work"
)

func Priorities(qc QueryContext) (res []work.IssuePriority, rerr error) {

	objectPath := "priority"

	var rawPriorities []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"statusColor"`
		Icon        string `json:"iconUrl"`
	}

	err := qc.Req.Get(objectPath, nil, &rawPriorities)
	if err != nil {
		rerr = err
		return
	}

	// the result comes back in priority order from HIGH (0) to LOW (length-1)
	// so we iterate backwards to make the highest first and the lowest last

	var order int64
	for i := len(rawPriorities) - 1; i >= 0; i-- {
		priority := rawPriorities[i]
		res = append(res, work.IssuePriority{
			ID:          work.NewIssuePriorityID(qc.CustomerID, "jira", priority.ID),
			CustomerID:  qc.CustomerID,
			Name:        priority.Name,
			Description: ps.Pointer(priority.Description),
			IconURL:     ps.Pointer(priority.Icon),
			Color:       ps.Pointer(priority.Color),
			Order:       int64(1 + order), // we use 0 for no order so offset by one to make the last != 0
			RefType:     "jira",
			RefID:       priority.ID,
		})
		order++
	}

	return
}
