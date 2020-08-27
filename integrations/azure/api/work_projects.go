package api

import (
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/work"
)

// FetchProjects gets the projects and sends them to the projchan channel
func (api *API) FetchProjects(projectsByName []string, excludedids []string, includedids []string) (res []*work.Project, err error) {
	url := `_apis/projects/`
	var r []projectResponse
	if err = api.getRequest(url, stringmap{"stateFilter": "all"}, &r); err != nil {
		return nil, err
	}
	var allProjects []*work.Project
	for _, p := range r {
		allProjects = append(allProjects, &work.Project{
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

	if len(projectsByName) != 0 {
		onlyInclude := projectsByName

		ok := map[string]bool{}
		for _, name := range onlyInclude {
			ok[name] = true
		}
		for _, repo := range allProjects {
			if !ok[repo.Name] {
				continue
			}
			res = append(res, repo)
		}
		api.logger.Info("projects", "found", len(allProjects), "projects_specified", len(onlyInclude), "result", len(res))
		return
	}

	if len(excludedids) != 0 && len(includedids) != 0 {

		var included []*work.Project
		{
			ok := map[string]bool{}
			for _, id := range includedids {
				ok[id] = true
			}
			for _, project := range allProjects {
				if !ok[project.RefID] {
					continue
				}
				included = append(included, project)
			}
		}

		excluded := map[string]bool{}
		for _, id := range excludedids {
			excluded[id] = true
		}

		filtered := map[string]*work.Project{}
		for _, project := range included {
			if excluded[project.RefID] {
				continue
			}
			filtered[project.RefID] = project
		}

		for _, project := range filtered {
			res = append(res, project)
		}

		api.logger.Info("projects", "found", len(allProjects), "excluded_definition", len(excludedids), "included_definition", len(includedids), "result", len(res))
		return
	}

	return
}
