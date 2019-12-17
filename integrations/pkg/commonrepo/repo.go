package commonrepo

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
)

type Repo struct {
	ID            string
	NameWithOwner string
	// DefaultBranch of the repo, could be empty if no commits yet. Used for getting commit_users
	DefaultBranch string
}

func (s Repo) GetID() string {
	return s.ID
}

func (s Repo) GetReadableID() string {
	return s.NameWithOwner
}

var _ repoprojects.RepoProject = (*Repo)(nil)

type ReposAll func(chan []Repo) error

func ReposAllSlice(reposAll ReposAll) (sl []Repo, rerr error) {
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

func reposToCommon(repo []Repo) (res []repoprojects.RepoProject) {
	for _, r := range repo {
		res = append(res, r)
	}
	return
}

func commonToRepos(common []repoprojects.RepoProject) (res []Repo) {
	for _, r := range common {
		res = append(res, r.(Repo))
	}
	return
}

// FilterConfig specifies the repos to use. JSON tags are used in config definitions.
type FilterConfig struct {
	OnlyIncludeNames []string `json:"repos"`
	ExcludedIDs      []string `json:"excluded_repos"`
	StopAfterN       int      `json:"stop_after_n"`
}

func Filter(logger hclog.Logger, repos []Repo, config FilterConfig) []Repo {
	res := repoprojects.Filter(logger, reposToCommon(repos), repoprojects.FilterConfig{
		OnlyIncludeReadableIDs: config.OnlyIncludeNames,
		ExcludedIDs:            config.ExcludedIDs,
		StopAfterN:             config.StopAfterN,
	})
	return commonToRepos(res)
}
