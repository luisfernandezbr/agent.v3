package commonrepo

import (
	"reflect"

	"github.com/hashicorp/go-hclog"
)

type Repo struct {
	ID            string
	NameWithOwner string
	// DefaultBranch of the repo, could be empty if no commits yet. Used for getting commit_users
	DefaultBranch string
}

type ReposAll func(interface{}, string, chan []Repo) error

func ReposAllSlice(qc interface{}, groupName string, reposAll ReposAll) (sl []Repo, rerr error) {
	res := make(chan []Repo)
	go func() {
		defer close(res)
		err := reposAll(qc, groupName, res)
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
}

func FilterRepos(logger hclog.Logger, repos []Repo, config interface{}) (res []Repo) {

	lconfig := getLocalConfig(config)

	if len(lconfig.Repos) != 0 {
		ok := map[string]bool{}
		for _, nameWithOwner := range lconfig.Repos {
			ok[nameWithOwner] = true
		}
		for _, repo := range repos {
			if !ok[repo.NameWithOwner] {
				continue
			}
			res = append(res, repo)
		}
		logger.Info("repos", "found", len(repos), "repos_specified", len(lconfig.Repos), "result", len(res))
		return
	}

	excluded := map[string]bool{}
	for _, id := range lconfig.ExcludedRepos {
		excluded[id] = true
	}

	filtered := map[string]Repo{}
	for _, repo := range repos {
		if excluded[repo.ID] {
			continue
		}
		filtered[repo.ID] = repo
	}

	logger.Info("repos", "found", len(repos), "excluded_definition", len(lconfig.ExcludedRepos), "result", len(filtered))
	for _, repo := range filtered {
		res = append(res, repo)
	}
	return
}

func getLocalConfig(conf interface{}) Config {

	t := reflect.ValueOf(conf)

	return Config{
		Repos:         t.FieldByName("Repos").Interface().([]string),
		ExcludedRepos: t.FieldByName("ExcludedRepos").Interface().([]string),
	}
}
