package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/agent.next/rpcdef"
	pjson "github.com/pinpt/go-common/json"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportCode() (err error) {

	projectids, err := s.processRepos()
	if err != nil {
		return err
	}
	if err = s.processUsers(projectids); err != nil {
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

func (s *Integration) processRepos() (projectids []string, err error) {
	var repos []*sourcecode.Repo
	if projectids, repos, err = s.api.FetchAllRepos(s.IncludedRepos, s.ExcludedRepoIDs); err != nil {
		return
	}
	var orgname string
	if s.Creds.Organization != nil {
		orgname = *s.Creds.Organization
	} else {
		orgname = *s.Creds.Collection
	}
	sender, err := s.orgSession.Session(sourcecode.RepoModelName.String(), orgname, orgname)
	if err != nil {
		return
	}
	sender.SetTotal(len(repos))
	var errors []string
	for _, repo := range repos {
		sender.Send(repo)
		if err := s.api.FetchPullRequests(repo.RefID, repo.Name, sender); err != nil {
			errors = append(errors, err.Error())
		}
		if err := s.ripSource(repo); err != nil {
			s.logger.Error("error with ripsrc in repo", "data", repo.Stringify())
		}
	}
	sender.Done()
	if len(errors) > 0 {
		err = fmt.Errorf("errors: %v", strings.Join(errors, ", "))
	}
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
	args.RefType = s.RefType.String()
	args.RepoID = s.api.IDs.CodeRepo(repo.RefID)
	args.URL = u.String()
	s.logger.Info("queueing repo for processing " + u.String())
	args.BranchURLTemplate = branchURLTemplate(repo.Name, s.Creds.URL)
	args.CommitURLTemplate = commitURLTemplate(repo.Name, s.Creds.URL)
	return s.agent.ExportGitRepo(args)
}

func (s *Integration) processUsers(projectids []string) error {

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

	var orgname string
	if s.Creds.Organization != nil {
		orgname = *s.Creds.Organization
	} else {
		orgname = *s.Creds.Collection
	}
	sender, err := s.orgSession.Session(sourcecode.UserModelName.String(), orgname, orgname)
	if err != nil {
		s.logger.Error("error creating sender session for sourcecode user")
		return err
	}
	for _, user := range projusers {
		if err := sender.Send(user); err != nil {
			s.logger.Error("error sending project user", "data", user.Stringify())
		}
	}
	sender.Done()
	return nil
}
