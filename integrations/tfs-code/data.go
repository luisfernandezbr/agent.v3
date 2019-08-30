package main

import (
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
	s.exportCommitUsers(repoids)
	s.exportPullRequestData(repoids)
	return nil
}

func (s *Integration) exportReposAndRipSrc() (refids []string, err error) {
	sender := objsender.NewNotIncremental(s.agent, sourcecode.RepoModelName.String())
	repos, err := s.api.FetchRepos()
	s.logger.Info("Fetched repos", "cound", len(repos))
	if err != nil {
		return
	}
	for _, repo := range repos {
		refids = append(refids, repo.RefID)
		if err := sender.Send(repo); err != nil {
			return nil, err
		}
		// workaround for itexico server
		ur := strings.Replace(repo.URL, "itxwin04:8080", "itxwin04.itexico.com:8080", 1)

		u, e := url.Parse(ur)
		if e != nil {
			return nil, e
		}
		u.User = url.UserPassword(s.conf.Username, s.conf.Password)
		args := rpcdef.GitRepoFetch{}
		args.RepoID = s.api.RepoID(repo.RefID)
		args.URL = ur
		args.CommitURLTemplate = commitURLTemplate(repo.Name, s.conf.URL)
		s.agent.ExportGitRepo(args)
	}
	return refids, sender.Done()
}

func (s *Integration) exportCommitUsers(repoids []string) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, commitusers.TableName)
	if err != nil {
		return err
	}
	for _, repoid := range repoids {
		usrs, err := s.api.FetchCommitUsers(repoid, sender.LastProcessed)
		if err != nil {
			return err
		}
		for _, u := range usrs {
			sender.Send(u)
		}
	}
	return sender.Done()
}

func (s *Integration) exportPullRequestData(repoids []string) error {
	var prs []*sourcecode.PullRequest
	var prrs []*sourcecode.PullRequestReview
	var prc []*sourcecode.PullRequestComment

	for _, repoid := range repoids {
		pr, prr, err := s.api.FetchPullRequests(repoid)
		if err != nil {
			return err
		}
		for _, p := range pr {
			cmts, err := s.api.FetchPullRequestComments(repoid, p.RefID)
			if err != nil {
				return err
			}
			prc = append(prc, cmts...)
		}
		prs = append(prs, pr...)
		prrs = append(prrs, prr...)
	}

	var sender *objsender.NotIncremental
	// Send pull requests
	sender = objsender.NewNotIncremental(s.agent, sourcecode.PullRequestModelName.String())
	for _, pr := range prs {
		sender.Send(pr)
	}
	if err := sender.Done(); err != nil {
		return err
	}
	// Send pull request reviews
	sender = objsender.NewNotIncremental(s.agent, sourcecode.PullRequestReviewModelName.String())
	for _, prr := range prrs {
		sender.Send(prr)
	}
	if err := sender.Done(); err != nil {
		return err
	}
	// Send pull request comments
	sender = objsender.NewNotIncremental(s.agent, sourcecode.PullRequestCommentModelName.String())
	for _, c := range prc {
		sender.Send(c)
	}

	return sender.Done()
}
