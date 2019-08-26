package api

import "time"

type projectsResponse struct {
	Components []*Project `json:"components"`
}

// FetchProjects ...
func (a *SonarqubeAPI) FetchProjects(fromDate time.Time) ([]*Project, error) {

	val := projectsResponse{}
	err := a.doRequest("GET", "/components/search?p=1&ps=500&qualifiers=TRK", fromDate, &val)
	if err != nil {
		return nil, err
	}
	var components []*Project
	for _, w := range val.Components {
		components = append(components, w)
	}
	return components, nil

}
