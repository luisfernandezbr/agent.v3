package azureapi

import (
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/work"
)

// FetchProjects gets the projects and sends them to the projchan channel
func (api *API) FetchProjects(projchan chan<- datamodel.Model) ([]string, error) {
	_, projids, err := api.fetchProjects(projchan)
	if err != nil {
		return nil, err
	}
	return projids, nil
}

func (api *API) fetchProjects(projchan chan<- datamodel.Model) ([]projectResponse, []string, error) {
	url := `_apis/projects/`
	var res []projectResponse
	if err := api.getRequest(url, stringmap{"stateFilter": "all"}, &res); err != nil {
		return nil, nil, err
	}
	var projids []string
	for _, p := range res {
		projids = append(projids, p.ID)
		projchan <- &work.Project{
			Active:      p.State == "wellFormed",
			CustomerID:  api.customerid,
			Description: &p.Description,
			Identifier:  p.Name,
			Name:        p.Name,
			RefID:       p.ID,
			RefType:     api.reftype,
			URL:         p.URL,
		}
	}
	return res, projids, nil
}
