package api

import (
	"net/url"

	"github.com/pinpt/integration-sdk/agent"

	pstrings "github.com/pinpt/go-common/v10/strings"
)

func ProjectsOnboard(qc QueryContext) (res []*agent.ProjectResponseProjects, rerr error) {

	objectPath := "project"

	params := url.Values{}
	params.Set("expand", "description")

	var rr []struct {
		Self        string `json:"self"`
		ID          string `json:"id"`
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"projectCategory"`
	}

	err := qc.Req.Get(objectPath, params, &rr)
	if err != nil {
		rerr = err
		return
	}

	for _, data := range rr {
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

	return
}
