package api

import (
	"net/url"

	"github.com/pinpt/integration-sdk/agent"

	pstrings "github.com/pinpt/go-common/v10/strings"
)

func ProjectsOnboardPage(
	qc QueryContext,
	paginationParams url.Values) (pi PageInfo, res []*agent.ProjectResponseProjects, rerr error) {

	objectPath := "project/search"
	params := paginationParams

	//params.Set("maxResults", "1") // for testing
	params.Set("expand", "description")

	qc.Logger.Debug("projects request", "params", params)

	var rr struct {
		Total      int  `json:"total"`
		MaxResults int  `json:"maxResults"`
		IsLast     bool `json:"isLast"`
		Values     []struct {
			Self        string `json:"self"`
			ID          string `json:"id"`
			Key         string `json:"key"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Category    struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"projectCategory"`
		} `json:"values"`
	}

	err := qc.Req.Get(objectPath, params, &rr)
	if err != nil {
		rerr = err
		return
	}

	pi.Total = rr.Total
	pi.MaxResults = rr.MaxResults
	if len(rr.Values) != 0 {
		pi.HasMore = !rr.IsLast
	}

	for _, data := range rr.Values {
		item := &agent.ProjectResponseProjects{}
		item.RefID = data.ID
		item.RefType = "jira"

		item.Name = data.Name
		item.Identifier = data.Key
		item.Active = true
		item.URL = data.Self

		item.Description = pstrings.Pointer(data.Description)
		if data.Category.Name != "" {
			item.Category = pstrings.Pointer(data.Category.Name)
		}

		res = append(res, item)

	}

	return pi, res, nil
}
