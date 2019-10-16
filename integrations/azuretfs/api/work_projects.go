package api

import (
	"github.com/pinpt/integration-sdk/work"
)

// FetchProjects gets the projects and sends them to the projchan channel
func (api *API) FetchProjects() (projects []*work.Project, err error) {
	url := `_apis/projects/`
	var res []projectResponse
	if err = api.getRequest(url, stringmap{"stateFilter": "all"}, &res); err != nil {
		return nil, err
	}
	for _, p := range res {
		projects = append(projects, &work.Project{
			Active:      p.State == "wellFormed",
			CustomerID:  api.customerid,
			Description: &p.Description,
			Identifier:  p.Name,
			Name:        p.Name,
			RefID:       p.ID,
			RefType:     api.reftype,
			URL:         p.URL,
		})
	}
	return
}
