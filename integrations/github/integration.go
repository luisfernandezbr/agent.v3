package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/pkg/ids"
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
	requestsMadeAtomic     *int64
	requestsBuffer         float64

	refType string

	clientManager *reqstats.ClientManager
	clients       reqstats.Clients
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
	qc.IsEnterprise = func() bool {
		return s.config.Enterprise
	}
	s.qc = qc

	return nil
}

type IntegrationConfig struct {
	// URL URL of instance if relevant
	URL string `json:"url"`
	// APIKey API Key for instance, if relevant
	APIKey string `json:"api_key"`
	// Organization Organization for instance, if relevant
	Organization string `json:"organization"`
	// Exclusions list of exclusions
	Exclusions []string `json:"exclusions"`
	// Exclusions list of inclusions
	Inclusions []string `json:"inclusions"`
}

type Config struct {
	APIURL                string
	APIURL3               string
	RepoURLPrefix         string
	Token                 string
	Organization          string
	ExcludedRepos         []string
	IncludedRepos         []string
	OnlyGit               bool
	StopAfterN            int
	Enterprise            bool
	Repos                 []string
	Concurrency           int
	TLSInsecureSkipVerify bool
}

type configDef struct {
	OnlyGit bool `json:"only_git"`

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

func (s *Integration) setIntegrationConfig(data rpcdef.IntegrationConfig) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}

	var inConfig IntegrationConfig
	var res Config
	var def configDef

	err := structmarshal.StructToStruct(data.Config, &inConfig)
	if err != nil {
		return err
	}
	err = structmarshal.StructToStruct(data.Config, &def)
	if err != nil {
		return err
	}
	if token, ok := data.Config["access_token"].(string); ok && token != "" {
		inConfig.APIKey = token
		inConfig.URL = "https://github.com"
	}
	if inConfig.APIKey == "" {
		return errors.New("missing api_key")
	}
	res.Token = inConfig.APIKey

	if inConfig.URL == "" {
		return errors.New("missing url")
	}
	purl := inConfig.URL

	if inConfig.Organization != "" {
		res.Organization = inConfig.Organization
	}
	res.ExcludedRepos = inConfig.Exclusions
	res.IncludedRepos = inConfig.Inclusions

	res.Repos = def.Repos
	res.OnlyGit = def.OnlyGit
	res.StopAfterN = def.StopAfterN

	{
		u, err := url.Parse(purl)
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
	requestsMade := int64(-1) // start with -1 to check quotas before 1 request
	s.requestsMadeAtomic = &requestsMade
	s.requestsBuffer = 0

	s.qc.APIURL = s.config.APIURL
	s.qc.APIURL3 = s.config.APIURL3
	s.qc.AuthToken = s.config.Token
	s.clientManager = reqstats.New(reqstats.Opts{
		Logger:                s.logger,
		TLSInsecureSkipVerify: s.config.TLSInsecureSkipVerify,
	})
	s.clients = s.clientManager.Clients
	s.qc.Clients = s.clients
	s.qc.Request = s.makeRequest

	if s.config.Enterprise {
		err := s.checkEnterpriseVersion()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Integration) Export(ctx context.Context, exportConfig rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	err := s.initWithConfig(exportConfig)
	if err != nil {
		return res, err
	}
	// we keep a request buffer for exports, but not onboarding or validation
	s.requestsBuffer = exportRequestBuffer

	projects, err := s.export(ctx)
	if err != nil {
		return res, err
	}

	res.Projects = projects

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
	var names []string
	for _, org := range res {
		names = append(names, org.Login)

	}
	s.logger.Info("found organizations", "orgs", names)
	return res, nil
}

type exportRepo struct {
	api.RepoWithDefaultBranch
}

func (s exportRepo) GetID() string {
	return s.RepoWithDefaultBranch.ID
}

func (s exportRepo) GetReadableID() string {
	return s.RepoWithDefaultBranch.NameWithOwner
}

func (s *Integration) export(ctx context.Context) (_ []rpcdef.ExportProject, rerr error) {

	orgs, err := s.getOrgs()
	if err != nil {
		rerr = err
		return
	}

	s.users, err = NewUsers(s, false)
	if err != nil {
		rerr = err
		return
	}
	// export all users in all organization, and when later encountering new users continue export
	err = s.users.ExportAllOrgUsers(orgs)
	if err != nil {
		rerr = err
		return
	}

	s.qc.UserLoginToRefIDFromCommit = s.users.LoginToRefIDFromCommit
	s.qc.ExportUserUsingFullDetails = s.users.ExportUserUsingFullDetails

	var unfilteredRepos []api.RepoWithDefaultBranch
	if len(orgs) > 0 {
		unfilteredRepos, rerr = s.getAllOrgRepos(orgs)
		if rerr != nil {
			return
		}
	} else {
		unfilteredRepos, rerr = s.getAllPersonalRepos(orgs)
		if rerr != nil {
			return
		}
	}

	var unfilteredReposIface []repoprojects.RepoProject
	for _, r := range unfilteredRepos {
		unfilteredReposIface = append(unfilteredReposIface, exportRepo{r})
	}

	filteredReposIface := repoprojects.Filter(s.logger, unfilteredReposIface, repoprojects.FilterConfig{
		OnlyIncludeReadableIDs: s.config.Repos,
		ExcludedIDs:            s.config.ExcludedRepos,
		IncludedIDs:            s.config.IncludedRepos,
		StopAfterN:             s.config.StopAfterN,
	})

	var filteredRepos []exportRepo
	for _, r := range filteredReposIface {
		filteredRepos = append(filteredRepos, r.(exportRepo))
	}

	// enable for pinpoint only
	if s.customerID == "ea63c052fd862a91" || s.customerID == "d05b8b6ef71e3575" || s.customerID == "14ea36c3b3cd0270" {
		err = s.registerWebhooks(filteredRepos)
		if err != nil {
			s.logger.Info("could not register webhooks", "err", err)
		}
	}

	repoSender, err := objsender.Root(s.agent, sourcecode.RepoModelName.String())
	if err != nil {
		rerr = err
		return
	}
	// we do not want to mark repo as exported until we export all pull request related data for it
	repoSender.SetNoAutoProgress(true)
	repoSender.SetTotal(len(filteredRepos))

	err = s.exportRepoMetadata(repoSender, orgs, filteredRepos)
	if err != nil {
		rerr = err
		return
	}

	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.logger
	processOpts.ProjectFn = func(ctx *repoprojects.ProjectCtx) error {
		repo := ctx.Project.(exportRepo)
		return s.exportRepo(ctx, repo.RepoWithDefaultBranch)
	}

	// we use s.config.Concurrency for request concurrency, set it here as well, since we don't have additional concurrency inside of repo, so process multiple repos concurrently
	// request concurrency is an additional safeguard in requests.go
	processOpts.Concurrency = s.config.Concurrency
	if processOpts.Concurrency < 1 {
		processOpts.Concurrency = 1
	}
	processOpts.Projects = filteredReposIface

	processOpts.IntegrationType = inconfig.IntegrationTypeSourcecode
	processOpts.CustomerID = s.customerID
	processOpts.RefType = s.refType
	processOpts.Sender = repoSender

	processor := repoprojects.NewProcess(processOpts)
	exportResult, err := processor.Run()
	if err != nil {
		rerr = err
		return
	}

	err = repoSender.Done()
	if err != nil {
		rerr = err
		return
	}

	err = s.users.Done()
	if err != nil {
		rerr = err
		return
	}

	s.logger.Info(s.clientManager.PrintStats())

	return exportResult, nil
}

func (s *Integration) registerWebhooks(repos []exportRepo) error {
	s.logger.Info("registering webhooks")

	url, err := s.agent.GetWebhookURL()
	if err != nil {
		return err
	}

	for _, repo := range repos {
		err := api.WebhookCreateIfNotExists(s.qc, repo.Repo(), url, webhookEvents)
		if err != nil {
			s.logger.Info("could not register webhooks for repo", "err", err, "repo", repo.NameWithOwner)
		}
	}

	return nil
}

func (s *Integration) getAllOrgRepos(orgs []api.Org) (res []api.RepoWithDefaultBranch, rerr error) {
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
			res = append(res, r)
		}
	}
	s.logger.Info("completed getting list of repos, total unfiltered", "c", len(res))
	return
}

func (s *Integration) getAllPersonalRepos(orgs []api.Org) (res []api.RepoWithDefaultBranch, rerr error) {
	s.logger.Info("getting a list of all personal repos")

	orgRepos, err := api.ReposAllSlice(s.qc.WithLogger(s.logger), api.Org{})
	if err != nil {
		rerr = err
		return
	}
	for _, r := range orgRepos {
		res = append(res, r)
	}

	s.logger.Info("completed getting list of personal repos, total unfiltered", "c", len(res))
	return
}

func commitURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/commit/@@@sha@@@"
}

func branchURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/tree/@@@branch@@@"
}

func (s *Integration) exportRepo(ctx *repoprojects.ProjectCtx, repo api.RepoWithDefaultBranch) error {
	logger := ctx.Logger
	if s.config.OnlyGit {
		logger.Warn("only_ripsrc flag passed, skipping export of data from github api, will not be exporting prs")

		// if only git do it here, otherwise wait till we export all prs per repo
		err := s.exportGit(repo.Repo(), nil)
		if err != nil {
			return err
		}

		return nil
	}

	// export a link between commit and github user
	err := s.exportCommitUsersForRepo(ctx, repo)
	if err != nil {
		return err
	}

	err = s.exportPullRequestsAndRelated(ctx, repo.Repo())
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

func (s *Integration) exportPullRequestsAndRelated(ctx *repoprojects.ProjectCtx, repo api.Repo) error {
	logger := ctx.Logger

	prSender, err := ctx.Session(sourcecode.PullRequestModelName)
	if err != nil {
		return err
	}
	prCommitsSender, err := ctx.Session(sourcecode.PullRequestCommitModelName)
	if err != nil {
		return err
	}

	prs, err := s.exportPullRequestsForRepo(logger, repo, prSender, prCommitsSender)
	if err != nil {
		return err
	}

	err = s.exportGit(repo, prs)
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
			s.logger.Error("could not export pull request comments", "err", err)
			//setErr(fmt.Errorf("could not export pull request comments: %v", err))
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.exportPullRequestsReviews(logger, pullRequestSender, repo, pullRequestsForReviews)
		if err != nil {
			s.logger.Error("could not export pull request reviews", "err", err)
			//setErr(fmt.Errorf("could not export pull request reviews: %v", err))
		}
	}()

	// set commits on the rp and then send the pr
	wg.Add(1)
	go func() {
		defer wg.Done()
		for prs := range pullRequestsForCommits {
			for _, pr := range prs {

				err := s.exportPRCommitsAddingToPR(logger, repo, pr, pullRequestSender, commitsSender)
				if err != nil {
					s.logger.Error("could not export pr commits", "err", err)
					//setErr(fmt.Errorf("could not export pr commits: %v", err))
					//return
				}
			}
		}
	}()
	wg.Wait()
	return
}

func (s *Integration) exportPRCommitsAddingToPR(logger hclog.Logger, repo api.Repo, pr api.PullRequest, pullRequestSender objsender.SessionCommon, commitsSender objsender.SessionCommon) error {
	logger = logger.With("repo", repo.NameWithOwner)
	commits, err := s.exportPullRequestCommits(logger, pr.RefID)
	if err != nil {
		return err
	}

	for _, c := range commits {
		pr.CommitShas = append(pr.CommitShas, c.Sha)
	}

	pr.CommitIds = ids.CodeCommits(s.qc.CustomerID, s.refType, pr.RepoID, pr.CommitShas)
	if len(pr.CommitShas) == 0 {
		logger.Info("found PullRequest with no commits (not setting BranchID)", "repo", pr.RepoID, "pr_ref_id", pr.RefID, "pr.url", pr.URL)
	} else {
		pr.BranchID = s.qc.BranchID(pr.RepoID, pr.BranchName, pr.CommitShas[0])
	}

	err = pullRequestSender.Send(pr)
	if err != nil {
		return fmt.Errorf("error sending pr: %v", err)
	}

	for _, c := range commits {
		c.BranchID = pr.BranchID
		err := commitsSender.Send(c)
		if err != nil {
			return fmt.Errorf("error sending commit: %v", err)
		}
	}

	return nil
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
