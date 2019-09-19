package commonrepo

import (
	"github.com/hashicorp/go-hclog"
)

type Repo struct {
	ID            string
	NameWithOwner string
	// DefaultBranch of the repo, could be empty if no commits yet. Used for getting commit_users
	DefaultBranch string
}

type ReposAll func(chan []Repo) error

func ReposAllSlice(qc interface{}, groupName string, reposAll ReposAll) (sl []Repo, rerr error) {
	res := make(chan []Repo)
	go func() {
		defer close(res)
		err := reposAll(res)
		if err != nil {
			rerr = err
		}
	}()
	for a := range res {
		for _, sub := range a {
			sl = append(sl, sub)
		}
	}
	return
}

type Config struct {
	Repos         []string
	ExcludedRepos []string
	StopAfterN    int
}

func FilterRepos(logger hclog.Logger, repos []Repo, config Config) (res []Repo) {

	if len(config.Repos) != 0 {
		ok := map[string]bool{}
		for _, nameWithOwner := range config.Repos {
			ok[nameWithOwner] = true
		}
		for _, repo := range repos {
			if !ok[repo.NameWithOwner] {
				continue
			}
			res = append(res, repo)
		}
		logger.Info("repos", "found", len(repos), "repos_specified", len(config.Repos), "result", len(res))
		return
	}

	excluded := map[string]bool{}
	for _, id := range config.ExcludedRepos {
		excluded[id] = true
	}

	filtered := map[string]Repo{}
	for _, repo := range repos {
		if excluded[repo.ID] {
			continue
		}
		filtered[repo.ID] = repo
	}

	logger.Info("repos", "found", len(repos), "excluded_definition", len(config.ExcludedRepos), "result", len(filtered))
	for _, repo := range filtered {
		res = append(res, repo)
	}

	if config.StopAfterN > 0 {
		res = res[:config.StopAfterN]
	}

	return
}
