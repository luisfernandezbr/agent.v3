package api

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchAllRepos gets the repos and filters the inclided and excludedids if any passed, and sends them to the repochan channel
func (api *API) FetchAllRepos(includedByName []string, excludedids []string, includedids []string) (projectids []string, repos []*sourcecode.Repo, err error) {
	rawrepos, err := api.fetchRepos("")
	if err != nil {
		return nil, nil, err
	}
	projectidmap := make(map[string]bool)
	var allRepos []*sourcecode.Repo
	for _, repo := range rawrepos {
		var reponame string
		if strings.HasPrefix(repo.Name, repo.Project.Name) {
			reponame = repo.Name
		} else {
			reponame = repo.Project.Name + "/" + repo.Name
		}
		allRepos = append(allRepos, &sourcecode.Repo{
			Active:        true,
			CustomerID:    api.customerid,
			DefaultBranch: strings.TrimPrefix(repo.DefaultBranch, "refs/heads/"),
			Name:          reponame,
			RefID:         repo.ID,
			RefType:       api.reftype,
			URL:           repo.RemoteURL,
		})
		projectidmap[repo.Project.ID] = true
	}

	if len(includedByName) != 0 {
		onlyInclude := includedByName

		ok := map[string]bool{}
		for _, nameWithOwner := range onlyInclude {
			ok[nameWithOwner] = true
		}
		for _, repo := range allRepos {
			if !ok[filepath.Base(repo.Name)] {
				continue
			}
			repos = append(repos, repo)
		}
		api.logger.Info("repos", "found", len(allRepos), "repos_specified", len(onlyInclude), "result", len(repos))
	} else if len(excludedids) != 0 && len(includedids) != 0 {
		var included []*sourcecode.Repo
		{
			ok := map[string]bool{}
			for _, id := range includedids {
				ok[id] = true
			}
			for _, repo := range allRepos {
				if !ok[repo.RefID] {
					continue
				}
				included = append(included, repo)
			}

		}

		excluded := map[string]bool{}
		for _, id := range excludedids {
			excluded[id] = true
		}

		filtered := map[string]*sourcecode.Repo{}
		for _, repo := range included {
			if excluded[repo.RefID] {
				continue
			}
			filtered[repo.RefID] = repo
		}

		for _, repo := range filtered {
			repos = append(repos, repo)
		}
		api.logger.Info("repos", "found", len(allRepos), "excluded_definition", len(excludedids), "included_definition", len(includedids), "result", len(repos))
	} else {
		for _, repo := range allRepos {
			repos = append(repos, repo)
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
