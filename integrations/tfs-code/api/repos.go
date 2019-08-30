package api

import (
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
func (a *TFSAPI) FetchRepos() (repos []*sourcecode.Repo, err error) {
	url := "/_apis/git/repositories/"
	var res []reposResponse
	if err = a.doRequest(url, nil, time.Time{}, &res); err != nil {
		return
	}
	for _, repo := range res {
		newrepo := &sourcecode.Repo{
			DefaultBranch: repo.DefaultBranch,
			Name:          repo.Name,
			RefID:         repo.ID,
			RefType:       a.reftype,
			URL:           repo.RemoteURL,
		}
		repos = append(repos, newrepo)
	}
	return
}
