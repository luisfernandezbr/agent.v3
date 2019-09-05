package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/go-common/datetime"

	"github.com/pinpt/agent.next/integrations/tfs-code/api"
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
	repoids, projids, err := s.exportReposAndRipSrc()
	if err != nil {
		return err
	}
	// exports api users and then commit users
	if err = s.exportUsers(projids, repoids); err != nil {
		return err
	}
	if err = s.exportPullRequestData(repoids); err != nil {
		return err
	}
	return nil
}

func (s *Integration) exportReposAndRipSrc() (repoids []string, projids []string, err error) {

	sender := objsender.NewNotIncremental(s.agent, sourcecode.RepoModelName.String())
	repos, projids, err := s.api.FetchRepos(s.conf.Repos, s.conf.Excluded)
	if err != nil {
		return
	}
	for _, repo := range repos {
		repoids = append(repoids, repo.RefID)
		if err := sender.Send(repo); err != nil {
			return nil, nil, err
		}
		u, e := url.Parse(repo.URL)
		if s.conf.OverrideGitHostName != "" {
			u.Host = s.conf.OverrideGitHostName
		}
		if e != nil {
			return nil, nil, e
		}
		u.User = url.UserPassword(s.creds.Username, s.creds.Password)
		args := rpcdef.GitRepoFetch{}
		args.RepoID = s.api.RepoID(repo.RefID)
		args.URL = u.String()
		args.CommitURLTemplate = commitURLTemplate(repo.Name, s.creds.URL)
		s.agent.ExportGitRepo(args)
	}
	return repoids, projids, sender.Done()
}

func (s *Integration) exportUsers(projids []string, repoids []string) error {

	sender := objsender.NewNotIncremental(s.agent, sourcecode.UserTable.String())
	usermap := make(map[string]*sourcecode.User)
	for _, proj := range projids {
		if err := s.api.FetchUsers(proj, usermap); err != nil {
			s.logger.Error("error fetching users", "err", err)
		}
	}
	for _, user := range usermap {
		sender.Send(user)
	}
	if err := sender.Done(); err != nil {
		return err
	}
	return s.exportCommitUsers(repoids, usermap)
}

func (s *Integration) exportCommitUsers(repoids []string, usermap map[string]*sourcecode.User) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, commitusers.TableName)
	if err != nil {
		return err
	}
	// Get a list of all the users in the commits
	// using a map we make sure we only get unique users
	rawusers := make(map[string]*api.RawCommitUser)
	for _, repoid := range repoids {
		if err := s.api.FetchCommitUsers(repoid, rawusers, sender.LastProcessed); err != nil {
			// log error and skip
			s.logger.Error("error fetching users", "err", err)
			continue
		}
	}
	// iterate through all the users and look for commit users which contain email addresses
	for _, u := range usermap {
		// if we have a match, set the email and send it to the agent
		if raw, ok := rawusers[u.Name]; ok {
			u.Email = &raw.Email
			if err := sender.Send(u); err != nil {
				return fmt.Errorf("error sending users. err: %v", err)
			}
		}
	}

	return sender.Done()
}

func (s *Integration) exportPullRequestData(repoids []string) error {

	prsender, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestModelName.String())
	if err != nil {
		return err
	}
	prrsender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestReviewModelName.String())
	prcsender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestCommentModelName.String())

	incremental := !prsender.LastProcessed.IsZero()
	for _, repoid := range repoids {
		prs, prrs, err := s.api.FetchPullRequests(repoid)
		if err != nil {
			// log error and skip
			s.logger.Error("error fetching pull requests and reviews", "err", err)
			continue
		}
		for _, pr := range prs {
			created := datetime.DateFromEpoch(pr.CreatedDate.Epoch)
			closed := datetime.DateFromEpoch(pr.ClosedDate.Epoch)
			// incremental, only send if this was created after the last processed date
			if !incremental || created.After(prsender.LastProcessed) {
				if err := prsender.Send(pr); err != nil {
					return fmt.Errorf("error sending pull requests. err: %v", err)
				}
			}
			// incremental, only send if this pr is still opened or was closed after the last processed date
			if !incremental || (pr.Status == sourcecode.PullRequestStatusOpen || (pr.ClosedDate.Epoch > 0 && closed.After(prsender.LastProcessed))) {
				cmts, err := s.api.FetchPullRequestComments(repoid, pr.RefID)
				if err != nil {
					// log error and skip
					s.logger.Error("error fetching pull requests comments", "err", err)
					continue
				}
				for _, prc := range cmts {
					updated := datetime.DateFromEpoch(prc.UpdatedDate.Epoch)
					if !incremental || updated.After(prsender.LastProcessed) {
						if err := prcsender.Send(prc); err != nil {
							return fmt.Errorf("error sending pull requests comments. err: %v", err)
						}
					}
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
