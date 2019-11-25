package api

import (
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/work"
)

// FetchProjects gets the projects and sends them to the projchan channel
func (api *API) FetchProjects(excludedids []string) (projects []*work.Project, err error) {
	url := `_apis/projects/`
	var res []projectResponse
	if err = api.getRequest(url, stringmap{"stateFilter": "all"}, &res); err != nil {
		return nil, err
	}
	for _, p := range res {
		if exists(p.ID, excludedids) {
			continue
		}
		projects = append(projects, &work.Project{
			Active:      p.State == "wellFormed",
			CustomerID:  api.customerid,
			Description: pstrings.Pointer(p.Description),
			Identifier:  p.Name,
			Name:        p.Name,
			RefID:       p.ID,
			RefType:     api.reftype,
			URL:         p.URL,
		})
	}
	return
}
