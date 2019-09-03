package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/rpcdef"
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
}

func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}

// setting higher to 1 starts returning the following error, even though the hourly limit is not used up yet.
// 403: You have triggered an abuse detection mechanism. Please wait a few minutes before you try again.
const maxRequestConcurrency = 1

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
	s.requestConcurrencyChan = make(chan bool, maxRequestConcurrency)

	return nil
}

type Config struct {
	APIURL        string
	APIURL3       string
	RepoURLPrefix string
	Token         string
	Organization  string
	ExcludedRepos []string
	OnlyGit       bool
	StopAfterN    int
	Enterprise    bool
	Repos         []string
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
			res.RepoURLPrefix = "https://" + strings.TrimPrefix(u.Host, "api.")
		} else {
			res.APIURL = urlAppend(u.String(), "api/graphql")
			res.APIURL3 = urlAppend(u.String(), "api/v3")
			res.RepoURLPrefix = u.String()
			res.Enterprise = true
		}

	}

	s.config = res

	return nil
}

func (s *Integration) ValidateConfig(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {

	rerr := func(err error) {
		res.Errors = append(res.Errors, err.Error())
	}

	err := s.initWithConfig(exportConfig)
	if err != nil {
		rerr(err)
		return
	}

	orgs, err := s.getOrgs()
	if err != nil {
		rerr(err)
		return
	}

	_, err = api.ReposAllSlice(s.qc, orgs[0])
	if err != nil {
		rerr(err)
		return
	}

	// TODO: return a repo and validate repo that repo can be cloned in agent

	return
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
	s.qc.APIURL3 = s.config.APIURL3

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

	for _, org := range orgs {
		err := s.exportOrganization(ctx, org)
		if err != nil {
			return err
		}
	}

	return s.users.Done()
}

func commitURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/commit/@@@sha@@@"
}

func branchURLTemplate(repo api.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/tree/@@@branch@@@"
}

func (s *Integration) filterRepos(logger hclog.Logger, repos []api.Repo) (res []api.Repo) {
	if len(s.config.Repos) != 0 {
		ok := map[string]bool{}
		for _, nameWithOwner := range s.config.Repos {
			ok[nameWithOwner] = true
		}
		for _, repo := range repos {
			if !ok[repo.NameWithOwner] {
				continue
			}
			res = append(res, repo)
		}
		logger.Info("repos", "found", len(repos), "repos_specified", len(s.config.Repos), "result", len(res))
		return
	}

	//all := map[string]bool{}
	//for _, repo := range repos {
	//	all[repo.ID] = true
	//}
	excluded := map[string]bool{}
	for _, id := range s.config.ExcludedRepos {
		// This does not work because we pass excluded repos for all orgs. But filterRepos is called separately per org.
		//if !all[id] {
		//	return fmt.Errorf("wanted to exclude non existing repo: %v", id)
		//}
		excluded[id] = true
	}

	filtered := map[string]api.Repo{}
	// filter excluded repos
	for _, repo := range repos {
		if excluded[repo.ID] {
			continue
		}
		filtered[repo.ID] = repo
	}

	logger.Info("repos", "found", len(repos), "excluded_definition", len(s.config.ExcludedRepos), "result", len(filtered))
	for _, repo := range filtered {
		res = append(res, repo)
	}
	return
}

func (s *Integration) exportOrganization(ctx context.Context, org api.Org) error {
	s.logger.Info("exporting organization", "login", org.Login)
	logger := s.logger.With("org", org.Login)

	repos, err := api.ReposAllSlice(s.qc.WithLogger(logger), org)
	if err != nil {
		return err
	}

	repos = s.filterRepos(logger, repos)

	if s.config.StopAfterN != 0 {
		// only leave 1 repo for export
		stopAfter := s.config.StopAfterN
		l := len(repos)
		if len(repos) > stopAfter {
			repos = repos[0:stopAfter]
		}
		logger.Info("stop_after_n passed", "v", stopAfter, "repos", l, "after", len(repos))
	}

	// queue repos for processing with ripsrc
	{

		for _, repo := range repos {
			u, err := url.Parse(s.config.RepoURLPrefix)
			if err != nil {
				return err
			}
			u.User = url.UserPassword(s.config.Token, "")
			u.Path = repo.NameWithOwner
			repoURL := u.String()

			args := rpcdef.GitRepoFetch{}
			args.RepoID = s.qc.RepoID(repo.ID)
			args.URL = repoURL
			args.CommitURLTemplate = commitURLTemplate(repo, s.config.RepoURLPrefix)
			args.BranchURLTemplate = branchURLTemplate(repo, s.config.RepoURLPrefix)
			err = s.agent.ExportGitRepo(args)
			if err != nil {
				return err
			}
		}
	}

	if s.config.OnlyGit {
		logger.Warn("only_ripsrc flag passed, skipping export of data from github api")
		return nil
	}

	// export repos
	{
		err := s.exportRepos(ctx, logger, org, repos)
		if err != nil {
			return err
		}
	}

	// export a link between commit and github user
	// This is much slower than the rest
	// for pinpoint takes 3.5m for initial, 47s for incremental
	{
		// higher concurrency does not make any real difference
		commitConcurrency := 1

		err := s.exportCommitUsers(logger, repos, commitConcurrency)
		if err != nil {
			return err
		}
	}

	pullRequestSender, err := objsender.NewIncrementalDateBased(s.agent, sourcecode.PullRequestModelName.String())
	if err != nil {
		return err
	}
	pullRequestCommentsSender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestCommentModelName.String())
	if err != nil {
		return err
	}
	pullRequestReviewsSender := objsender.NewNotIncremental(s.agent, sourcecode.PullRequestReviewModelName.String())
	if err != nil {
		return err
	}

	for _, repo := range repos {
		err := s.exportPullRequestsForRepo(logger, repo, pullRequestSender, pullRequestCommentsSender, pullRequestReviewsSender)
		if err != nil {
			return err
		}
	}

	err = pullRequestSender.Done()
	if err != nil {
		return err
	}
	err = pullRequestCommentsSender.Done()
	if err != nil {
		return err
	}
	err = pullRequestReviewsSender.Done()
	if err != nil {
		return err
	}

	return nil
}

func (s *Integration) exportPullRequestsForRepo(logger hclog.Logger, repo api.Repo,
	pullRequestSender *objsender.IncrementalDateBased,
	commentsSender *objsender.NotIncremental,
	reviewSender *objsender.NotIncremental) (rerr error) {

	logger = logger.With("repo", repo.NameWithOwner)
	logger.Info("exporting")

	// export changed pull requests
	var pullRequestsErr error
	pullRequestsInitial := make(chan []api.PullRequest)
	go func() {
		defer close(pullRequestsInitial)
		err := s.exportPullRequestsRepo(logger, repo, pullRequestsInitial, pullRequestSender.LastProcessed)
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
		err := s.exportPullRequestsComments(logger, commentsSender, pullRequestsForComments)
		if err != nil {
			setErr(err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.exportPullRequestsReviews(logger, reviewSender, pullRequestsForReviews)
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
				pr.CommitShas = commits
				pr.CommitIds = ids.CodeCommits(s.qc.CustomerID, s.refType, pr.RepoID, commits)
				if len(pr.CommitShas) == 0 {
					panic("no commits found in pr, github does not allow you to create empty pr so this should not be possible")
				}
				pr.BranchID = s.qc.BranchID(pr.RepoID, pr.BranchName, pr.CommitShas[0])
				err = pullRequestSender.Send(pr)
				if err != nil {
					setErr(err)
					return
				}
			}
		}
	}()
	wg.Wait()
	return
}
