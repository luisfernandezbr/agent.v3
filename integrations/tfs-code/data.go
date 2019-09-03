package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/agent.next/pkg/commitusers"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func urlAppend(p1, p2 string) string {
	return strings.TrimSuffix(p1, "/") + "/" + p2
}

func commitURLTemplate(reponame, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, reponame) + "/commit/@@@sha@@@"
}

func (s *Integration) export() error {
	repoids, err := s.exportReposAndRipSrc()
	if err != nil {
		return err
	}
	if err = s.exportCommitUsers(repoids); err != nil {
		return err
	}
	if err = s.exportPullRequestData(repoids); err != nil {
		return err
	}
	return nil
}

func (s *Integration) exportReposAndRipSrc() (refids []string, err error) {
	sender := objsender.NewNotIncremental(s.agent, sourcecode.RepoModelName.String())
	repos, err := s.api.FetchRepos(s.repos, s.excluded)
	s.logger.Info("Fetched repos", "count", len(repos))
	if err != nil {
		return
	}
	for _, repo := range repos {
		refids = append(refids, repo.RefID)
		if err := sender.Send(repo); err != nil {
			return nil, err
		}

		s.logger.Info("git repo url", "url", repo.URL)
		// workaround for itexico server
		// ur := strings.Replace(repo.URL, "itxwin04:8080", "itxwin04.itexico.com:8080", 1)

		u, e := url.Parse(repo.URL)
		if e != nil {
			return nil, e
		}
		u.User = url.UserPassword(s.creds.Username, s.creds.Password)
		args := rpcdef.GitRepoFetch{}
		args.RepoID = s.api.RepoID(repo.RefID)
		args.URL = repo.URL
		args.CommitURLTemplate = commitURLTemplate(repo.Name, s.creds.URL)
		s.agent.ExportGitRepo(args)
	}
	return refids, sender.Done()
}

func (s *Integration) exportCommitUsers(repoids []string) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, commitusers.TableName)
	if err != nil {
		return err
	}
	usermap := make(map[string]*sourcecode.User)
	for _, repoid := range repoids {
		if err := s.api.FetchCommitUsers(repoid, usermap, sender.LastProcessed); err != nil {
			return fmt.Errorf("error fetching users. err: %v", err)
		}
	}
	for _, u := range usermap {
		if err := sender.Send(u); err != nil {
			return fmt.Errorf("error sending users. err: %v", err)
		}
	}

	return sender.Done()
}

func (s *Integration) exportPullRequestData(repoids []string) error {

	prsender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestModelName.String())
	prrsender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestReviewModelName.String())
	prcsender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestCommentModelName.String())

	for _, repoid := range repoids {
		prs, prrs, err := s.api.FetchPullRequests(repoid)
		if err != nil {
			return fmt.Errorf("error fetching pull requests and reviews. err: %v", err)
		}
		for _, pr := range prs {
			if err := prsender.Send(pr); err != nil {
				return fmt.Errorf("error sending pull requests. err: %v", err)
			}
			cmts, err := s.api.FetchPullRequestComments(repoid, pr.RefID)
			if err != nil {
				return fmt.Errorf("error fetching pull requests comments. err: %v", err)
			}
			for _, prc := range cmts {
				if err := prcsender.Send(prc); err != nil {
					return fmt.Errorf("error sending pull requests comments. err: %v", err)
				}
			}
		}
		for _, prr := range prrs {
			if err := prrsender.Send(prr); err != nil {
				return fmt.Errorf("error sending pull request reviews comments. err: %v", err)
			}
		}
	}

	if err := prsender.Done(); err != nil {
		return err
	}
	if err := prcsender.Done(); err != nil {
		return err
	}
	if err := prrsender.Done(); err != nil {
		return err
	}
	return nil
}
