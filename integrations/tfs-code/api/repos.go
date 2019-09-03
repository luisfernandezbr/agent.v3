package api

import (
	"path/filepath"
	"time"

	"github.com/pinpt/integration-sdk/sourcecode"
)

type reposResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Project struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
		State       string `json:"state"`
		Revision    int64  `json:"revision"`
	} `json:"project"`
	DefaultBranch string `json:"defaultBranch"`
	RemoteURL     string `json:"remoteUrl"`
}

// FetchRepos calls the repo api returns a list of sourcecode.Repo
func (a *TFSAPI) FetchRepos(included []string, excludedids []string) (repos []*sourcecode.Repo, err error) {
	url := "/_apis/git/repositories/"
	var res []reposResponse
	if err = a.doRequest(url, nil, time.Time{}, &res); err != nil {
		return
	}
	for _, repo := range res {
		if len(excludedids) > 0 && exists(repo.ID, excludedids) {
			continue
		}
		// 1. check if there are any in included
		// 2. check if the repo name is in the included
		// 3. check if the repo name, with the collection name, is in the included
		if len(included) == 0 || exists(filepath.Base(repo.Name), included) || exists(filepath.Join(a.creds.Collection, repo.Name), included) {
			newrepo := &sourcecode.Repo{
				DefaultBranch: repo.DefaultBranch,
				Name:          repo.Name,
				RefID:         repo.ID,
				RefType:       a.reftype,
				URL:           repo.RemoteURL,
			}
			repos = append(repos, newrepo)
		}
	}
	return
}
