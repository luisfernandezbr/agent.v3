package main

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
	"github.com/pinpt/agent.next/pkg/commitusers"
	"github.com/pinpt/agent.next/pkg/objsender"
)

func (s *Integration) exportCommitUsers(logger hclog.Logger, repos []api.Repo, concurrency int) error {

	wg := sync.WaitGroup{}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range reposToChan(repos, 0) {
				err := s.exportCommitsForRepoDefaultBranch(logger, s.commitUserSender, repo)
				if err != nil {
					panic(err)
				}
			}
		}()
	}
	wg.Wait()

	return nil
}

func reposToChan(sl []api.Repo, maxToReturn int) chan api.Repo {
	res := make(chan api.Repo)
	go func() {
		defer close(res)
		for i, a := range sl {
			if maxToReturn != 0 {
				if i == maxToReturn {
					return
				}
			}
			res <- a
		}
	}()
	return res
}

func (s *Integration) exportCommitsForRepoDefaultBranch(logger hclog.Logger, userSender *objsender.IncrementalDateBased, repo api.Repo) error {
	logger.Info("exporting commits (to get users)", "repo_id", repo.ID, "repo_name", repo.NameWithOwner)

	if repo.DefaultBranch == "" {
		return nil
	}

	err := s.exportCommitsForRepoBranch(logger, userSender, repo, repo.DefaultBranch)
	if err != nil {
		return err
	}

	return nil
}

func (s *Integration) exportCommitsForRepoBranch(logger hclog.Logger, userSender *objsender.IncrementalDateBased, repo api.Repo, branchName string) error {
	logger.Info("exporting commits for branch", "repo_id", repo.ID, "repo_name", repo.NameWithOwner)

	return api.PaginateNewerThan(logger, userSender.LastProcessed, func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.CommitsPage(s.qc,
			repo.ID,
			branchName,
			parameters,
		)
		if err != nil {
			return pi, err
		}

		logger.Info("got commits page", "repo", repo.ID, "l", len(res))

		for _, commit := range res {
			validate := func(u commitusers.CommitUser, kind string) error {
				err := u.Validate()
				if err != nil {
					return fmt.Errorf("commit data does not have proper %v repo: %v commit: %v %v", kind, repo.NameWithOwner, commit.CommitHash, err)
				}
				return nil
			}

			author := commitusers.CommitUser{}
			author.CustomerID = s.customerID
			author.Name = commit.AuthorName
			author.Email = commit.AuthorEmail
			author.SourceID = s.qc.UserEmailMap[author.Email]

			committer := commitusers.CommitUser{}
			committer.CustomerID = s.customerID
			committer.Name = commit.CommitterName
			committer.Email = commit.CommitterEmail
			committer.SourceID = s.qc.UserEmailMap[author.Email]

			err := validate(author, "author")
			if err != nil {
				// TODO: some commits don't have associated emails, but that is not an error we are logging it here to validate in more details in the future
				// if it's all ok can remove the warning as well
				s.logger.Warn("commit user", "err", err)
			} else {
				err := userSender.SendMap(author.ToMap())
				if err != nil {
					return pi, err
				}
			}

			err = validate(committer, "commiter")
			if err != nil {
				s.logger.Warn("commit user", "err", err)
			} else {
				err := userSender.SendMap(committer.ToMap())
				if err != nil {
					return pi, err
				}
			}
		}

		return pi, nil
	})

}
