package azureapi

import (
	"fmt"
	purl "net/url"
	"path/filepath"
	"strings"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchAllRepos gets the repos and filters the inclided and excludedids if any passed, and sends them to the repochan channel
func (api *API) FetchAllRepos(included []string, excludedids []string, repochan chan<- datamodel.Model) ([]string, error) {
	rawrepos, err := api.fetchRepos("")
	if err != nil {
		return nil, err
	}
	projectidmap := make(map[string]bool)
	for _, repo := range rawrepos {
		if len(excludedids) > 0 && exists(repo.ID, excludedids) {
			continue
		}
		// 1. check if there are any in included
		// 2. check if the repo name is in the included
		if len(included) == 0 || exists(filepath.Base(repo.Name), included) {
			repochan <- &sourcecode.Repo{
				Active:        true,
				CustomerID:    api.customerid,
				DefaultBranch: strings.Replace(repo.DefaultBranch, "refs/heads/", "", 1),
				Name:          repo.Name,
				RefID:         repo.ID,
				RefType:       api.reftype,
				URL:           repo.RemoteURL,
			}
			projectidmap[repo.Project.ID] = true
		}
	}
	var projectids = []string{}
	for projid := range projectidmap {
		projectids = append(projectids, projid)
	}
	return projectids, nil
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
