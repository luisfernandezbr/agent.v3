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
	if err = sender.SetTotal(len(repos)); err != nil {
		s.logger.Error("error setting total repos on processRepos", "err", err)
	}
	var errors []string
	for _, repo := range repos {
		if err = sender.Send(repo); err != nil {
			s.logger.Error("error sending repo", "repo_id", repo.RefID, "err", err)
			return
		}
		var fetchprs []rpcdef.GitRepoFetchPR
		if fetchprs, err = s.api.FetchPullRequests(repo.RefID, repo.Name, sender); err != nil {
			errors = append(errors, err.Error())
		}
		if err := s.ripSource(repo, fetchprs); err != nil {
			s.logger.Error("error with ripsrc in repo", "data", repo.Stringify())
		}
	}
	if err = sender.Done(); err != nil {
		errors = append(errors, err.Error())
	}
	if len(errors) > 0 {
		err = fmt.Errorf("errors: %v", strings.Join(errors, ", "))
	}
	return
}

func (s *Integration) appendCredentials(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if s.OverrideGitHostName != "" {
		u.Host = s.OverrideGitHostName
	}
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(s.Creds.Username, s.Creds.Password)
	return u.String(), nil
}

func (s *Integration) ripSource(repo *sourcecode.Repo, fetchprs []rpcdef.GitRepoFetchPR) error {

	repoURL, err := s.appendCredentials(repo.URL)
	if err != nil {
		return err
	}

	args := rpcdef.GitRepoFetch{}
	args.RepoID = s.api.IDs.CodeRepo(repo.RefID)
	args.UniqueName = repo.Name
	args.RefType = s.RefType.String()
	args.URL = repoURL
	args.CommitURLTemplate = commitURLTemplate(repo.Name, s.Creds.URL)
	args.BranchURLTemplate = branchURLTemplate(repo.Name, s.Creds.URL)
	args.PRs = fetchprs
	s.logger.Info("queueing repo for processing " + repo.URL)

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
	return sender.Done()
}
