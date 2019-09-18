package azureapi

import (
	"github.com/pinpt/integration-sdk/work"
)

func (api *API) fetchProjects() ([]projectResponse, error) {
	url := `_apis/projects/`
	var res []projectResponse
	if err := api.getRequest(url, stringmap{"stateFilter": "all"}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *API) FetchProjects(projchan chan work.Project) ([]string, error) {
	projs, err := api.fetchProjects()
	if err != nil {
		return nil, err
	}
	var projids []string
	for _, p := range projs {
		projchan <- work.Project{
			Active:      p.State == "wellFormed",
			CustomerID:  api.customerid,
			Description: &p.Description,
			Identifier:  p.Name, // ??
			Name:        p.Name,
			RefID:       p.ID,
			RefType:     api.reftype,
			URL:         p.URL,
		}
		projids = append(projids, p.ID)
	}
	return projids, nil
}
