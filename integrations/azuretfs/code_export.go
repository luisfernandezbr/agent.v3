package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/agent.next/integrations/azuretfs/api"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-common/datamodel"
	pjson "github.com/pinpt/go-common/json"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportCode() error {
	repoids, projectids, err := s.processRepos()
	if err != nil {
		return err
	}
	if err := s.processPullRequests(repoids); err != nil {
		return err
	}
	if err := s.processUsers(repoids, projectids); err != nil {
		return err
	}
	return nil
}

func urlAppend(p1, p2 string) string {
	return strings.TrimSuffix(p1, "/") + "/" + p2
}

func commitURLTemplate(reponame, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, reponame) + "/commit/@@@sha@@@"
}

func branchURLTemplate(reponame, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, reponame) + "/tree/@@@branch@@@"
}

func stringify(i interface{}) string {
	return pjson.Stringify(i)
}

func (s *Integration) processRepos() (repoids []string, projectids []string, err error) {
	sender := objsender.NewNotIncremental(s.agent, sourcecode.RepoModelName.String())
	defer sender.Done()

	items, done := api.AsyncProcess("repos", s.logger, func(model datamodel.Model) {
		repo := model.(*sourcecode.Repo)
		if err := sender.Send(repo); err != nil {
			s.logger.Error("error sending "+repo.GetModelName().String(), "err", err)
		}
		repoids = append(repoids, repo.RefID)
		if err := s.ripSource(repo); err != nil {
			s.logger.Error("error with ripsrc in repo", "data", repo.Stringify())
		}
	})
	if projectids, err = s.api.FetchAllRepos(s.IncludedRepos, s.ExcludedRepoIDs, items); err != nil {
		return
	}
	close(items)
	<-done
	return
}

func (s *Integration) ripSource(repo *sourcecode.Repo) error {
	u, err := url.Parse(repo.URL)
	if s.OverrideGitHostName != "" {
		u.Host = s.OverrideGitHostName
	}
	if err != nil {
		return err
	}
	u.User = url.UserPassword(s.Creds.Username, s.Creds.Password)
	args := rpcdef.GitRepoFetch{}
	args.RepoID = s.api.IDs.CodeRepo(repo.RefID)
	args.UniqueName = repo.Name
	args.RefType = s.RefType.String()
	args.URL = u.String()
	s.logger.Info("queueing repo for processing " + u.String())
	args.BranchURLTemplate = branchURLTemplate(repo.Name, s.Creds.URL)
	args.CommitURLTemplate = commitURLTemplate(repo.Name, s.Creds.URL)
	return s.agent.ExportGitRepo(args)
}

func (s *Integration) processPullRequests(repoids []string) error {
	senderprs, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestModelName.String())
	if err != nil {
		return err
	}
	defer senderprs.Done()
	senderprrs, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestReviewModelName.String())
	if err != nil {
		return err
	}
	defer senderprrs.Done()
	senderprcs, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestCommitModelName.String())
	if err != nil {
		return err
	}
	defer senderprcs.Done()
	senderprcms, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestCommentModelName.String())
	if err != nil {
		return err
	}
	defer senderprcms.Done()
	prchan, prdone := api.AsyncProcess("pull requests", s.logger, func(model datamodel.Model) {
		if err := senderprs.Send(model); err != nil {
			s.logger.Error("error sending "+model.GetModelName().String(), "err", err)
		}
	})
	prrchan, prrdone := api.AsyncProcess("pull request reviews", s.logger, func(model datamodel.Model) {
		if err := senderprrs.Send(model); err != nil {
			s.logger.Error("error sending "+model.GetModelName().String(), "err", err)
		}
	})
	prcchan, prcdone := api.AsyncProcess("pull request comments", s.logger, func(model datamodel.Model) {
		if err := senderprcs.Send(model); err != nil {
			s.logger.Error("error sending "+model.GetModelName().String(), "err", err)
		}
	})
	prcmhan, prmdone := api.AsyncProcess("pull request commits", s.logger, func(model datamodel.Model) {
		if err := senderprcms.Send(model); err != nil {
			s.logger.Error("error sending "+model.GetModelName().String(), "err", err)
		}
	})
	var errors []string
	for _, repoid := range repoids {
		if err := s.api.FetchPullRequests(repoid, senderprs.LastProcessed, prchan, prrchan, prcchan, prcmhan); err != nil {
			errors = append(errors, err.Error())
			continue
		}
	}
	close(prchan)
	close(prrchan)
	close(prcchan)
	close(prcmhan)
	<-prdone
	<-prrdone
	<-prcdone
	<-prmdone
	if errors != nil {
		return fmt.Errorf("error fetching pull requests. err %s", strings.Join(errors, ", "))
	}
	return nil
}

func (s *Integration) processUsers(repoids []string, projectids []string) error {
	senderproj := objsender.NewNotIncremental(s.agent, sourcecode.UserModelName.String())
	defer senderproj.Done()

	projusers := make(map[string]*sourcecode.User)
	for _, projid := range projectids {
		teamids, err := s.api.FetchTeamIDs(projid)
		if err != nil {
			return err
		}
		if err := s.api.FetchSourcecodeUsers(projid, teamids, projusers); err != nil {
			return err
		}
	}
	// Project Users:
	for _, user := range projusers {
		if err := senderproj.Send(user); err != nil {
			s.logger.Error("error sending project user", "data", user.Stringify())
		}
	}
	return nil
}
