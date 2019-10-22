package api

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchAllRepos gets the repos and filters the inclided and excludedids if any passed, and sends them to the repochan channel
func (api *API) FetchAllRepos(included []string, excludedids []string) (projectids []string, repos []*sourcecode.Repo, err error) {
	rawrepos, err := api.fetchRepos("")
	if err != nil {
		return nil, nil, err
	}
	projectidmap := make(map[string]bool)
	for _, repo := range rawrepos {
		if exists(repo.ID, excludedids) {
			continue
		}
		// 1. check if there are any in included
		// 2. check if the repo name is in the included
		if len(included) == 0 || exists(filepath.Base(repo.Name), included) {
			var reponame string
			if strings.HasPrefix(repo.Name, repo.Project.Name) {
				reponame = repo.Name
			} else {
				reponame = repo.Project.Name + "/" + repo.Name
			}
			repos = append(repos, &sourcecode.Repo{
				Active:        true,
				CustomerID:    api.customerid,
				DefaultBranch: strings.Replace(repo.DefaultBranch, "refs/heads/", "", 1),
				Name:          reponame,
				RefID:         repo.ID,
				RefType:       api.reftype,
				URL:           repo.RemoteURL,
			})
			projectidmap[repo.Project.ID] = true
		}
	}
	for projid := range projectidmap {
		projectids = append(projectids, projid)
	}
	return
}

func (api *API) fetchRepos(projid string) ([]reposResponse, error) {
	// projid is optional, can be ""
	u := fmt.Sprintf(`%s/_apis/git/repositories/`, url.PathEscape(projid))
	var res []reposResponse
	if err := api.getRequest(u, nil, &res); err != nil {
		return nil, err
	}
	return res, nil
}
