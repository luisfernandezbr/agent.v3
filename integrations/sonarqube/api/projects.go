package api

import (
	"time"

	"github.com/pinpt/integration-sdk/codequality"
)

type projectsResponse struct {
	Components []*struct {
		ID           string `json:"id"`
		Key          string `json:"key"`
		Name         string `json:"name"`
		Organization string `json:"organization"`
		Qualifier    string `json:"qualifier"`
		Project      string `json:"project"`
	} `json:"components"`
}

// FetchProjects ...
func (a *SonarqubeAPI) FetchProjects(fromDate time.Time) ([]*codequality.Project, error) {

	val := projectsResponse{}
	err := a.doRequest("GET", "/components/search?p=1&ps=500&qualifiers=TRK", fromDate, &val)
	if err != nil {
		return nil, err
	}
	var projects []*codequality.Project
	for _, proj := range val.Components {
		projects = append(projects, &codequality.Project{
			Identifier: proj.Key,
			Name:       proj.Name,
			RefID:      proj.ID,
			RefType:    "sonarqube",
		})
	}
	return projects, nil

}
