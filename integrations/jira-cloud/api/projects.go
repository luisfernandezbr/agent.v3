package api

import (
	"net/url"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/integration-sdk/work"
)

func ProjectsPage(
	qc QueryContext,
	paginationParams url.Values) (pi PageInfo, res []*work.Project, _ error) {

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
			ID          string `json:"id"`
			Key         string `json:"key"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Category    struct {
				ID string `json:"id"`
			} `json:"projectCategory"`
		} `json:"values"`
	}

	err := qc.Request(objectPath, params, &rr)
	if err != nil {
		return pi, res, err
	}

	pi.Total = rr.Total
	pi.MaxResults = rr.MaxResults
	if len(rr.Values) != 0 {
		pi.HasMore = !rr.IsLast
	}

	for _, data := range rr.Values {
		item := &work.Project{}
		item.CustomerID = qc.CustomerID
		item.RefID = data.ID
		item.RefType = "jira"
		item.URL = qc.common().ProjectURL(data.Key)
		item.Name = data.Name
		item.Identifier = data.Key
		item.Active = true
		item.Description = pstrings.Pointer(data.Description)
		if data.Category.ID != "" {
			item.Category = pstrings.Pointer(data.Category.ID)
		}
		res = append(res, item)
	}

	return pi, res, nil
}
