package api

import (
	"net/url"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/integration-sdk/work"
)

func Projects(qc QueryContext) (res []*work.Project, rerr error) {
	qc.Logger.Debug("projects request")

	objectPath := "project"

	params := url.Values{}
	params.Set("expand", "description")
	var rr []struct {
		ID          string `json:"id"`
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    struct {
			ID string `json:"id"`
			//Name string `json:"name"`
		} `json:"projectCategory"`
	}

	err := qc.Request(objectPath, params, &rr)
	if err != nil {
		rerr = err
		return
	}

	for _, data := range rr {
		item := &work.Project{}
		item.CustomerID = qc.CustomerID
		item.RefID = data.ID
		item.RefType = "jira"
		item.URL = qc.Common().ProjectURL(data.Key)
		item.Name = data.Name
		item.Identifier = data.Key
		item.Active = true
		item.Description = pstrings.Pointer(data.Description)
		if data.Category.ID != "" {
			item.Category = pstrings.Pointer(data.Category.ID)
		}
		res = append(res, item)
	}

	return
}
