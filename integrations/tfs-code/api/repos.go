package api

import (
	"path/filepath"
	"time"

	"github.com/pinpt/integration-sdk/sourcecode"
)

type RawProjectInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	State       string `json:"state"`
	Revision    int64  `json:"revision"`
}

type reposResponse struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	URL           string         `json:"url"`
	Project       RawProjectInfo `json:"project"`
	DefaultBranch string         `json:"defaultBranch"`
	RemoteURL     string         `json:"remoteUrl"`
}

// FetchRepos calls the repo api returns a list of sourcecode.Repo and unique project ids
func (a *TFSAPI) FetchRepos(included []string, excludedids []string) (repos []*sourcecode.Repo, projectids []string, err error) {
	url := "/_apis/git/repositories/"
	var res []reposResponse
	if err = a.doRequest(url, nil, time.Time{}, &res); err != nil {
		return
	}
	projectmap := make(map[string]bool)
	for _, repo := range res {
		if len(excludedids) > 0 && exists(repo.ID, excludedids) {
			continue
		}
		// 1. check if there are any in included
		// 2. check if the repo name is in the included
		// 3. check if the repo name, with the collection name, is in the included
		if len(included) == 0 || exists(filepath.Base(repo.Name), included) || exists(filepath.Join(a.creds.Collection, repo.Name), included) {
			newrepo := &sourcecode.Repo{
				RefID:         repo.ID,
				RefType:       a.reftype,
				CustomerID:    a.customerid,
				Active:        true,
				DefaultBranch: repo.DefaultBranch,
				Name:          repo.Name,
				URL:           repo.RemoteURL,
			}
			repos = append(repos, newrepo)
			projectmap[repo.Project.ID] = true
		}
	}
	for projid := range projectmap {
		projectids = append(projectids, projid)
	}
	return
}
