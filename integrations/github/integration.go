package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/agent/pkg/integrationid"
	"github.com/pinpt/agent/pkg/reqstats"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/rpcdef"
)

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc    api.QueryContext
	users *Users

	config Config

	requestConcurrencyChan chan bool

	refType string

	clientManager *reqstats.ClientManager
	clients       reqstats.Clients

	exportPullRequestsForRepoFailed   []RepoError
	exportPullRequestsForRepoFailedMu sync.Mutex
}

func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	s.refType = "github"

	qc := api.QueryContext{}
	qc.Logger = s.logger
	qc.Request = s.makeRequest
	qc.IsEnterprise = func() bool {
		return s.config.Enterprise
	}
	s.qc = qc

	return nil
}

type Config struct {
	APIURL                string
	APIURL3               string
	RepoURLPrefix         string
	Token                 string
	Organization          string
	ExcludedRepos         []string
	OnlyGit               bool
	StopAfterN            int
	Enterprise            bool
	Repos                 []string
	Concurrency           int
	TLSInsecureSkipVerify bool
}

type configDef struct {
	URL      string `json:"url"`
	APIToken string `json:"apitoken"`

	// ExcludedRepos are the repos to exclude from processing. This is based on github repo id.
	ExcludedRepos []string `json:"excluded_repos"`
	OnlyGit       bool     `json:"only_git"`

	// Organization specifies the organization to export. By default all account organization are exported. Set this to export only one.
	Organization string `json:"organization"`

	// Repos specifies the repos to export. By default all repos are exported not including the ones from ExcludedRepos. This option overrides this.
	// Use github nameWithOwner for this field.
	// Example: user1/repo1
	Repos []string `json:"repos"`

	// StopAfterN stops exporting after N number of repos for testing and dev purposes
	StopAfterN int `json:"stop_after_n"`

	// Concurrency set higher concurrency.
	// github.com
	// setting higher to 1 starts returning the following error, even though the hourly limit is not used up yet.
	// 403: You have triggered an abuse detection mechanism. Please wait a few minutes before you try again.
	// github enterprise
	// Needs testing.
	Concurrency int `json:"concurrency"`
}

func (s *Integration) setIntegrationConfig(data map[string]interface{}) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}
	var def configDef
	err := structmarshal.MapToStruct(data, &def)
	if err != nil {
		return err
	}

	if def.URL == "" {
		return rerr("url is missing")
	}
	if def.APIToken == "" {
		return rerr("apitoken is missing")
	}
	var res Config
	res.Token = def.APIToken
	res.Organization = def.Organization
	res.ExcludedRepos = def.ExcludedRepos
	res.Repos = def.Repos
	res.OnlyGit = def.OnlyGit
	res.StopAfterN = def.StopAfterN

	{
		u, err := url.Parse(def.URL)
		if err != nil {
			return rerr("url is invalid: %v", err)
		}
		// allow both http(s)://github.com and http(s)://api.github.com
		if u.Host == "github.com" || u.Host == "api.github.com" {
			u, _ = url.Parse("https://api.github.com")
		}
		if u.Host == "api.github.com" {
			res.APIURL = urlAppend(u.String(), "graphql")
			res.APIURL3 = u.String()
			res.RepoURLPrefix = "https://" + strings.TrimPrefix(u.Host, "api.")
		} else {
			res.APIURL = urlAppend(u.String(), "api/graphql")
			res.APIURL3 = urlAppend(u.String(), "api/v3")
			res.RepoURLPrefix = u.String()
			res.Enterprise = true
			// TODO: make it configurable in admin
			res.TLSInsecureSkipVerify = true
		}
	}

	res.Concurrency = def.Concurrency
	if def.Concurrency == 0 {
		if !res.Enterprise {
			// github.com starts to return errors with more than 1 concurrency
			res.Concurrency = 1
		} else {
			// 2x faster with concurrency 10 than 1
			res.Concurrency = 10
		}
	}
	s.logger.Info("Using concurrency", "concurrency", res.Concurrency)

	s.config = res

	return nil
}

func urlAppend(p1, p2 string) string {
	return strings.TrimSuffix(p1, "/") + "/" + p2
}

func (s *Integration) initWithConfig(exportConfig rpcdef.ExportConfig) error {
	s.customerID = exportConfig.Pinpoint.CustomerID
	s.qc.CustomerID = s.customerID
	s.qc.RefType = "github"
	err := s.setIntegrationConfig(exportConfig.Integration)
	if err != nil {
		return err
	}

	s.requestConcurrencyChan = make(chan bool, s.config.Concurrency)

	s.qc.APIURL3 = s.config.APIURL3
	s.qc.AuthToken = s.config.Token
	s.clientManager = reqstats.New(reqstats.Opts{
		Logger:                s.logger,
		TLSInsecureSkipVerify: s.config.TLSInsecureSkipVerify,
	})
	s.clients = s.clientManager.Clients
	s.qc.Clients = s.clients

	if s.config.Enterprise {
		err := s.checkEnterpriseVersion()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Integration) Export(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	err := s.initWithConfig(exportConfig)
	if err != nil {
		return res, err
	}

	err = s.export(ctx)
	if err != nil {
		return res, err
	}

	return res, nil
}

func (s *Integration) getOrgs() (res []api.Org, _ error) {
	if s.config.Organization != "" {
		s.logger.Info("only_organization passed", "org", s.config.Organization)
		return []api.Org{{Login: s.config.Organization}}, nil
	}
	var err error
	if !s.config.Enterprise {
		res, err = api.OrgsAll(s.qc)
		if err != nil {
			return nil, err
		}
	} else {
		res, err = api.OrgsEnterpriseAll(s.qc)
		if err != nil {
			return nil, err
		}
	}
	if len(res) == 0 {
		return nil, errors.New("no organizations found in account")
	}
	var names []string
	for _, org := range res {
		names = append(names, org.Login)

	}
	s.logger.Info("found organizations", "orgs", names)
	return res, nil
}

func (s *Integration) export(ctx context.Context) error {

	orgs, err := s.getOrgs()
	if err != nil {
		return err
	}

	// export all users in all organization, and when later encountering new users continue export
	s.users, err = NewUsers(s, orgs)
	if err != nil {
		return err
	}

	s.qc.UserLoginToRefID = s.users.LoginToRefID
	s.qc.UserLoginToRefIDFromCommit = s.users.LoginToRefIDFromCommit

	unfilteredRepos, err := s.getAllRepos(orgs)
	if err != nil {
		return err
	}
	var unfilteredReposIface []repoprojects.RepoProject
	for _, r := range unfilteredRepos {
		unfilteredReposIface = append(unfilteredReposIface, r)
	}

	filteredReposIface := repoprojects.Filter(s.logger, unfilteredReposIface, repoprojects.FilterConfig{
		OnlyIncludeReadableIDs: s.config.Repos,
		ExcludedIDs:            s.config.ExcludedRepos,
		StopAfterN:             s.config.StopAfterN,
	})

	var filteredRepos []Repo
	for _, r := range filteredReposIface {
		filteredRepos = append(filteredRepos, r.(Repo))
	}

	repoSender, err := objsender.Root(s.agent, sourcecode.RepoModelName.String())
	if err != nil {
		return err
	}
	// we do not want to mark repo as exported until we export all pull request related data for it
	repoSender.SetNoAutoProgress(true)
	repoSender.SetTotal(len(filteredRepos))

	err = s.exportRepoMetadata(repoSender, orgs, filteredRepos)
	if err != nil {
		return err
	}

	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.logger
	processOpts.ProjectFn = func(ctx *repoprojects.ProjectCtx) error {
		repo := ctx.Project.(Repo)
		return s.exportRepo(ctx.Logger, repoSender, repo)
	}

	// we use s.config.Concurrency for request concurrency, set it here as well, since we don't have additional concurrency inside of repo, so process multiple repos concurrently
	// request concurrency is an additional safeguard in requests.go
	processOpts.Concurrency = s.config.Concurrency
	if processOpts.Concurrency < 1 {
		processOpts.Concurrency = 1
	}
	processOpts.Projects = filteredReposIface

	processOpts.IntegrationType = integrationid.TypeSourcecode
	processOpts.CustomerID = s.customerID
	processOpts.RefType = s.refType
	processOpts.Sender = repoSender

	processor := repoprojects.NewProcess(processOpts)
	_, err = processor.Run()
	if err != nil {
		return err
	}

	err = repoSender.Done()
	if err != nil {
		return err
	}

	err = s.users.Done()
	if err != nil {
		return err
	}

	s.logger.Debug(s.clientManager.PrintStats())

	if len(s.exportPullRequestsForRepoFailed) == 0 {
		s.logger.Info("Completed github export without errors")
	} else {
		s.logger.Error("Github export failed when getting prs and other data for the following repos")
		for _, re := range s.exportPullRequestsForRepoFailed {
			s.logger.Error("Failed getting repo data", "repo", re.Repo.NameWithOwner, "err", re.Err)
		}
	}

	return nil
}

type Repo struct {
	api.Repo
}

func (s Repo) GetID() string {
	return s.Repo.ID
}

func (s Repo) GetReadableID() string {
	return s.Repo.NameWithOwner
}

func (s *Integration) getAllRepos(orgs []api.Org) (res []Repo, rerr error) {
	s.logger.Info("getting a list of all repos")
	for _, org := range orgs {
		logger := s.logger.With("org", org.Login)
		s.logger.Info("getting repos for org")
		orgRepos, err := api.ReposAllSlice(s.qc.WithLogger(logger), org)
		if err != nil {
			rerr = err
			return
		}
		for _, r := range orgRepos {
			res = append(res, Repo{r})
		}
	}
	s.logger.Info("completed getting list of repos, total unfiltered", "c", len(res))
	return
}

func commitURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/commit/@@@sha@@@"
}

func branchURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/tree/@@@branch@@@"
}

func (s *Integration) exportRepo(logger hclog.Logger, repoSender *objsender.Session, repo Repo) error {
	if s.config.OnlyGit {
		logger.Warn("only_ripsrc flag passed, skipping export of data from github api, will not be exporting prs")

		// if only git do it here, otherwise wait till we export all prs per repo
		err := s.exportGit(repo.Repo, nil)
		if err != nil {
			return err
		}

		return nil
	}

	// export a link between commit and github user
	err := s.exportCommitUsersForRepo(logger, repo, repoSender)
	if err != nil {
		return err
	}

	err = s.exportPullRequestsAndRelated(logger, repo, repoSender)
	if err != nil {
		return err
	}

	return nil
}

func (s *Integration) exportGit(repo api.Repo, prs []PRMeta) error {
	repoURL, err := getRepoURL(s.config.RepoURLPrefix, url.UserPassword(s.config.Token, ""), repo.NameWithOwner)
	if err != nil {
		return err
	}

	args := rpcdef.GitRepoFetch{}
	args.RepoID = s.qc.RepoID(repo.ID)
	args.UniqueName = repo.NameWithOwner
	args.RefType = s.refType
	args.URL = repoURL
	args.CommitURLTemplate = commitURLTemplate(repo, s.config.RepoURLPrefix)
	args.BranchURLTemplate = branchURLTemplate(repo, s.config.RepoURLPrefix)
	for _, pr := range prs {
		if pr.LastCommitSHA == "" {
			s.logger.Error("pr.LastCommitSHA is missing", "repo", repo.NameWithOwner, "pr", pr.URL)
			continue
		}
		args.PRs = append(args.PRs, rpcdef.GitRepoFetchPR(pr))
	}

	err = s.agent.ExportGitRepo(args)
	if err != nil {
		return err
	}
	return nil
}

type PRMeta rpcdef.GitRepoFetchPR

type RepoError struct {
	Repo api.Repo
	Err  error
}

func (s *Integration) exportPullRequestsAndRelated(logger hclog.Logger, repo Repo, repoSender *objsender.Session) error {

	prSender, err := repoSender.Session(sourcecode.PullRequestModelName.String(), repo.ID, repo.NameWithOwner)
	if err != nil {
		return err
	}
	prCommitsSender, err := repoSender.Session(sourcecode.PullRequestCommitModelName.String(), repo.ID, repo.NameWithOwner)
	if err != nil {
		return err
	}

	prs, err := s.exportPullRequestsForRepo(logger, repo.Repo, prSender, prCommitsSender)
	if err != nil {
		return err
	} else {
		err = s.exportGit(repo.Repo, prs)
		if err != nil {
			return err
		}

		err = prSender.Done()
		if err != nil {
			return err
		}
		err = prCommitsSender.Done()
		if err != nil {
			return err
		}

	}

	err = repoSender.IncProgress()
	if err != nil {
		return err
	}
	return nil
}

func (s *Integration) exportPullRequestsForRepo(
	logger hclog.Logger, repo api.Repo,
	pullRequestSender *objsender.Session,
	commitsSender *objsender.Session) (res []PRMeta, rerr error) {

	logger = logger.With("repo", repo.NameWithOwner)
	logger.Info("exporting")

	// export changed pull requests
	var pullRequestsErr error
	pullRequestsInitial := make(chan []api.PullRequest)
	go func() {
		defer close(pullRequestsInitial)
		err := s.exportPullRequestsRepo(logger, repo, pullRequestSender, pullRequestsInitial, pullRequestSender.LastProcessedTime())
		if err != nil {
			pullRequestsErr = err
		}
	}()

	// export comments, reviews, commits concurrently
	pullRequestsForComments := make(chan []api.PullRequest, 10)
	pullRequestsForReviews := make(chan []api.PullRequest, 10)
	pullRequestsForCommits := make(chan []api.PullRequest, 10)

	var errMu sync.Mutex
	setErr := func(err error) {
		logger.Error("failed repo export", "e", err)
		errMu.Lock()
		defer errMu.Unlock()
		if rerr == nil {
			rerr = err
		}
		// drain all pull requests on error
		for range pullRequestsForComments {
		}
		for range pullRequestsForReviews {
		}
		for range pullRequestsForCommits {
		}
	}

	go func() {
		for item := range pullRequestsInitial {
			for _, pr := range item {
				meta := PRMeta{}
				repoID := s.qc.RepoID(repo.ID)
				meta.ID = s.qc.PullRequestID(repoID, pr.RefID)
				meta.RefID = pr.RefID
				meta.URL = pr.URL
				meta.BranchName = pr.BranchName
				meta.LastCommitSHA = pr.LastCommitSHA
				res = append(res, meta)
			}
			pullRequestsForComments <- item
			pullRequestsForReviews <- item
			pullRequestsForCommits <- item
		}
		close(pullRequestsForComments)
		close(pullRequestsForReviews)
		close(pullRequestsForCommits)

		if pullRequestsErr != nil {
			setErr(pullRequestsErr)
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.exportPullRequestsComments(logger, pullRequestSender, pullRequestsForComments)
		if err != nil {
			setErr(err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.exportPullRequestsReviews(logger, pullRequestSender, pullRequestsForReviews)
		if err != nil {
			setErr(err)
		}
	}()

	// set commits on the rp and then send the pr
	wg.Add(1)
	go func() {
		defer wg.Done()
		for prs := range pullRequestsForCommits {
			for _, pr := range prs {

				commits, err := s.exportPullRequestCommits(logger, pr.RefID)
				if err != nil {
					setErr(err)
					return
				}

				for _, c := range commits {
					pr.CommitShas = append(pr.CommitShas, c.Sha)
				}

				pr.CommitIds = ids.CodeCommits(s.qc.CustomerID, s.refType, pr.RepoID, pr.CommitShas)
				if len(pr.CommitShas) == 0 {
					logger.Info("found PullRequest with no commits (ignoring it)", "repo", repo.NameWithOwner, "pr_ref_id", pr.RefID, "pr.url", pr.URL)
				} else {
					pr.BranchID = s.qc.BranchID(pr.RepoID, pr.BranchName, pr.CommitShas[0])
				}

				err = pullRequestSender.Send(pr)
				if err != nil {
					setErr(err)
					return
				}

				for _, c := range commits {
					c.BranchID = pr.BranchID
					err := commitsSender.Send(c)
					if err != nil {
						setErr(err)
						return
					}
				}

			}
		}
	}()
	wg.Wait()
	return
}

func getRepoURL(repoURLPrefix string, user *url.Userinfo, nameWithOwner string) (string, error) {
	u, err := url.Parse(repoURLPrefix)
	if err != nil {
		return "", err
	}
	u.User = user
	u.Path = nameWithOwner
	return u.String(), nil
}
