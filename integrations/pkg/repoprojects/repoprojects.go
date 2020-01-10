// Package repoprojects contains the common filtering code for the main integration projects (or repos).
package repoprojects

import "github.com/hashicorp/go-hclog"

type RepoProject interface {
	// ID returns an internal database id. Not necessary readable.
	// TODO: rename to RefID
	GetID() string
	// ReadableID returns human readable id or name. In case of repos it would be "org/repo_name" or in case of jira project "EXAM".
	GetReadableID() string
}

type FilterConfig struct {
	OnlyIncludeReadableIDs []string
	ExcludedIDs            []string
	StopAfterN             int
}

func Filter(logger hclog.Logger, repos []RepoProject, config FilterConfig) (res []RepoProject) {

	if len(config.OnlyIncludeReadableIDs) != 0 {
		onlyInclude := config.OnlyIncludeReadableIDs

		ok := map[string]bool{}
		for _, nameWithOwner := range onlyInclude {
			ok[nameWithOwner] = true
		}
		for _, repo := range repos {
			if !ok[repo.GetReadableID()] {
				continue
			}
			res = append(res, repo)
		}
		logger.Info("projects", "found", len(repos), "projects_specified", len(onlyInclude), "result", len(res))
		return
	}

	excluded := map[string]bool{}
	for _, id := range config.ExcludedIDs {
		excluded[id] = true
	}

	filtered := map[string]RepoProject{}
	for _, repo := range repos {
		if excluded[repo.GetID()] {
			continue
		}
		filtered[repo.GetID()] = repo
	}

	logger.Info("projects", "found", len(repos), "excluded_definition", len(config.ExcludedIDs), "result", len(filtered))

	for _, repo := range filtered {
		res = append(res, repo)
	}

	if config.StopAfterN > 0 {
		// only leave 1 repo/project for export
		stopAfter := config.StopAfterN
		l := len(res)
		if l > stopAfter {
			res = res[0:stopAfter]
		}

		logger.Info("stop_after_n passed", "v", stopAfter, "projects", l, "after", len(repos))
	}

	return
}
