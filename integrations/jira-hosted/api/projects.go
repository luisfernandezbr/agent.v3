package api

import (
	"github.com/pinpt/go-datamodel/work"
)

func Projects(qc QueryContext) ([]*work.Project, error) {
	qc.Logger.Debug("projects request")

	objectPath := "project"

	var rr []struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}

	err := qc.Request(objectPath, nil, &rr)
	if err != nil {
		return nil, err
	}

	qc.Logger.Debug("result", "projects", rr)

	return nil, nil
}

/*
	//params.Set("maxResults", "1") // for testing
	//params.Set("expand", "description")

	//qc.Logger.Debug("projects request", "params", params)

	var rr struct {
		Total      int  `json:"total"`
		MaxResults int  `json:"maxResults"`
		IsLast     bool `json:"isLast"`
		Values     []struct {
			ID          string `json:"id"`
			Key         string `json:"key"`
			Name        string `json:"name"`
			Active      bool   `json:"active"`
			Description string `json:"description"`
			Category    struct {
				ID string `json:"id"`
			} `json:"projectCategory"`
		} `json:"values"`
	}

	err := qc.Request(objectPath, params, &rr)
	if err != nil {
		return nil, err
	}

	for _, data := range rr.Values {
		item := &work.Project{}
		item.CustomerID = qc.CustomerID
		item.RefID = data.ID
		item.RefType = "jira"

		item.Name = data.Name
		item.Identifier = data.Key
		item.Active = data.Active
		item.Description = pstrings.Pointer(data.Description)
		if data.Category.ID != "" {
			item.Category = pstrings.Pointer(data.Category.ID)
		}
		res = append(res, item)
	}
(/)
	return nil, err
}

*/
