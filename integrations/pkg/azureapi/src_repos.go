package azureapi

import (
	"fmt"
	purl "net/url"
	"path/filepath"

	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchAllRepos calls the repo api returns a list of sourcecode.Repo and unique project ids
func (api *API) FetchAllRepos(included []string, excludedids []string) ([]*sourcecode.Repo, []string, error) {
	rawrepos, err := api.fetchRepos("")
	if err != nil {
		return nil, nil, err
	}
	projectidmap := make(map[string]bool)
	var repos = []*sourcecode.Repo{}
	for _, repo := range rawrepos {
		if len(excludedids) > 0 && exists(repo.ID, excludedids) {
			continue
		}
		// 1. check if there are any in included
		// 2. check if the repo name is in the included
		if len(included) == 0 || exists(filepath.Base(repo.Name), included) {
			repos = append(repos, &sourcecode.Repo{
				RefID:         repo.ID,
				RefType:       api.reftype,
				CustomerID:    api.customerid,
				Active:        true,
				DefaultBranch: repo.DefaultBranch,
				Name:          repo.Name,
				URL:           repo.RemoteURL,
			})
			projectidmap[repo.Project.ID] = true
		}
	}
	var projectids = []string{}
	for projid := range projectidmap {
		projectids = append(projectids, projid)
	}
	return repos, projectids, nil
}

func (api *API) fetchRepos(projid string) ([]reposResponse, error) {
	// projid is optional, can be ""
	url := fmt.Sprintf(`%s/_apis/git/repositories/`, purl.PathEscape(projid))
	var res []reposResponse
	if err := api.getRequest(url, nil, &res); err != nil {
		return nil, err
	}
	return res, nil
}
