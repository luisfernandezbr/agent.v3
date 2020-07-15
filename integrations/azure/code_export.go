package main

import (
	"net/url"
	"strings"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"golang.org/x/exp/errors/fmt"

	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/rpcdef"
	pjson "github.com/pinpt/go-common/v10/json"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportCode() (exportResults []rpcdef.ExportProject, rerr error) {
	s.logger.Info("exporting code")
	projectids, exportResults, err := s.processRepos()
	if err != nil {
		rerr = err
		return
	}
	if err = s.processUsers(projectids); err != nil {
		rerr = err
		return
	}

	return exportResults, nil
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

func (s *Integration) processRepos() (projectIDs []string, exportResults []rpcdef.ExportProject, rerr error) {
	s.logger.Info("processing repos, fetching all repos")
	ids, reposDetails, err := s.api.FetchAllRepos(s.Repos, s.ExcludedRepoIDs, s.IncludedRepoIDs)
	if err != nil {
		rerr = fmt.Errorf("error fetching repos: %v", err)
		return
	}
	s.logger.Info("done fetching all repos")
	projectIDs = ids

	var orgname string
	if s.Creds.Organization != "" {
		orgname = s.Creds.Organization
	} else {
		orgname = s.Creds.CollectionName
	}

	sender, err := s.orgSession.Session(sourcecode.RepoModelName.String(), orgname, orgname)
	if err != nil {
		rerr = err
		return
	}

	if err := sender.SetTotal(len(reposDetails)); err != nil {
		rerr = err
		return
	}

	sender.SetNoAutoProgress(true)

	for _, repo := range reposDetails {
		if err = sender.Send(repo); err != nil {
			rerr = err
			return
		}
	}

	var repos []Repo
	for _, repo := range reposDetails {
		repos = append(repos, Repo{repo})
	}
	var reposIface []repoprojects.RepoProject
	for _, repo := range repos {
		reposIface = append(reposIface, repo)
	}
	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.logger
	processOpts.ProjectFn = func(ctx *repoprojects.ProjectCtx) error {
		repo := ctx.Project.(Repo)
		fetchprs, err := s.api.FetchPullRequests(ctx, repo.RefID, repo.Name, sender)
		if err != nil {
			return err
		}
		return s.ripSource(repo.Repo, fetchprs)
	}

	processOpts.Concurrency = s.Concurrency
	processOpts.Projects = reposIface
	processOpts.IntegrationType = inconfig.IntegrationTypeSourcecode
	processOpts.CustomerID = s.customerid
	processOpts.RefType = s.RefType.String()
	processOpts.Sender = sender

	processor := repoprojects.NewProcess(processOpts)
	exportResults, err = processor.Run()
	if err != nil {
		rerr = err
		return
	}
	if err = sender.Done(); err != nil {
		rerr = err
		return
	}

	return
}

type Repo struct {
	*sourcecode.Repo
}

func (s Repo) GetID() string {
	return s.RefID
}

func (s Repo) GetReadableID() string {
	return s.Name
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
	if s.Creds.Organization != "" {
		orgname = s.Creds.Organization
	} else {
		orgname = s.Creds.CollectionName
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
