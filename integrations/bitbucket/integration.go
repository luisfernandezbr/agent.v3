package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	pjson "github.com/pinpt/go-common/v10/json"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/bitbucket/api"
	"github.com/pinpt/agent/integrations/pkg/commiturl"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/pkg/commitusers"
	"github.com/pinpt/agent/pkg/ids2"
	"github.com/pinpt/agent/pkg/oauthtoken"
	"github.com/pinpt/agent/pkg/reqstats"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/agent/rpcdef"
)

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}

func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}

type Config struct {
	commonrepo.FilterConfig
	URL                string `json:"url"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	OnlyGit            bool   `json:"only_git"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`

	Exclusions []string `json:"exclusions"`
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc api.QueryContext

	config Config

	refType  string
	UseOAuth bool
	oauth    *oauthtoken.Manager

	clientManager *reqstats.ClientManager
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	s.refType = "bitbucket"

	s.qc = api.QueryContext{
		Logger: s.logger,
	}

	return nil
}

func (s *Integration) ValidateConfig(ctx context.Context, exportConfig rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {

	rerr := func(err error) {
		res.Errors = append(res.Errors, err.Error())
	}

	err := s.initWithConfig(exportConfig)
	if err != nil {
		rerr(err)
		return
	}

	res.ServerVersion = "cloud"

	err = api.AreUserCredentialsValid(s.qc)
	if err != nil {
		rerr(err)
		return
	}

	return
}

func (s *Integration) Export(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ExportResult, rerr error) {

	err := s.initWithConfig(exportConfig)
	if err != nil {
		rerr = err
		return
	}

	projects, err := s.export(ctx)
	if err != nil {
		rerr = err
		return
	}

	res.Projects = projects

	return
}

func (s *Integration) initWithConfig(config rpcdef.ExportConfig) error {
	err := s.setConfig(config)
	if err != nil {
		return err
	}

	var oauth *oauthtoken.Manager

	s.qc.BaseURL = s.config.URL
	s.qc.CustomerID = config.Pinpoint.CustomerID
	s.qc.Logger = s.logger
	s.qc.RefType = s.refType
	s.customerID = config.Pinpoint.CustomerID

	if s.UseOAuth {
		oauth, err = oauthtoken.New(s.logger, s.agent)
		if err != nil {
			return err
		}
		s.oauth = oauth
	}

	s.clientManager, err = reqstats.New(reqstats.Opts{
		Logger:                s.logger,
		TLSInsecureSkipVerify: s.config.InsecureSkipVerify,
	})
	if err != nil {
		return err
	}

	{
		opts := api.RequesterOpts{}
		opts.Logger = s.logger
		opts.APIURL = s.config.URL + "/2.0"
		opts.Username = s.config.Username
		opts.Password = s.config.Password
		opts.UseOAuth = s.UseOAuth
		opts.OAuth = oauth
		opts.Agent = s.agent
		opts.HTTPClient = s.clientManager.Clients.TLSInsecure
		requester := api.NewRequester(opts)

		s.qc.Request = requester.Request
		s.qc.IDs = ids2.New(s.customerID, s.refType)
	}

	return nil
}

func (s *Integration) setConfig(config rpcdef.ExportConfig) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg+" "+pjson.Stringify(config.Integration.Config), args...)
	}
	var def Config
	err := structmarshal.MapToStruct(config.Integration.Config, &def)
	if err != nil {
		return err
	}

	s.UseOAuth = config.UseOAuth
	if s.UseOAuth {
		def.URL = "https://api.bitbucket.org"
	} else {
		if def.URL == "" {
			return rerr("url is missing")
		}
		if def.Username == "" {
			return rerr("username is missing")
		}
		if def.Password == "" {
			return rerr("password is missing")
		}
	}
	s.config = def
	return nil
}

func (s *Integration) export(ctx context.Context) (exportResults []rpcdef.ExportProject, rerr error) {

	exportResults, rerr = s.exportAllRepos(ctx)
	if rerr != nil {
		return
	}

	s.logger.Info(s.clientManager.PrintStats())

	return
}

func (s *Integration) exportAllRepos(ctx context.Context) (_ []rpcdef.ExportProject, err error) {

	repos, err := commonrepo.ReposAllSlice(func(res chan []commonrepo.Repo) error {
		return api.ReposAll(s.qc, res)
	})
	if err != nil {
		return
	}

	repos = commonrepo.Filter(s.logger, repos, s.config.FilterConfig)

	if s.config.OnlyGit {
		s.logger.Warn("only_ripsrc flag passed, skipping export of data from bitbucket api")
		for _, repo := range repos {
			err = s.exportGit(repo, nil)
			if err != nil {
				return
			}
		}
		return
	}

	if err := s.exportWorkspacesUsers(ctx, repos); err != nil {
		return nil, err
	}

	repoSender, err := objsender.Root(s.agent, sourcecode.RepoModelName.String())
	if err != nil {
		return
	}

	// export repos
	err = s.exportRepos(ctx, s.logger, repoSender, repos)
	if err != nil {
		return
	}

	s.logger.Info("exporting repos", "len", len(repos))

	if err = repoSender.SetTotal(len(repos)); err != nil {
		return
	}

	var reposIface []repoprojects.RepoProject
	for _, repo := range repos {
		reposIface = append(reposIface, repo)
	}

	repoSender.SetNoAutoProgress(true)
	repoSender.SetTotal(len(reposIface))

	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.logger
	processOpts.ProjectFn = func(ctx *repoprojects.ProjectCtx) error {
		repo := ctx.Project.(commonrepo.Repo)
		return s.exportRepoChildren(ctx, repo)
	}

	processOpts.Concurrency = 1
	processOpts.Projects = reposIface

	processOpts.IntegrationType = inconfig.IntegrationTypeSourcecode
	processOpts.CustomerID = s.customerID
	processOpts.RefType = s.refType
	processOpts.Sender = repoSender

	processor := repoprojects.NewProcess(processOpts)
	exportResult, err := processor.Run()
	if err != nil {
		return
	}

	err = repoSender.Done()
	if err != nil {
		return
	}

	return exportResult, nil
}

func filterWorkspaces(repos []commonrepo.Repo) (workspaces map[string]interface{}) {

	workspaces = make(map[string]interface{})

	for _, repo := range repos {
		repoNameSeparated := strings.Split(repo.NameWithOwner, "/")
		if len(repoNameSeparated) == 0 {
			continue
		}

		workspaceName := repoNameSeparated[0]
		workspaces[workspaceName] = nil
	}
	return
}

func (s *Integration) exportWorkspacesUsers(ctx context.Context, repos []commonrepo.Repo) error {

	workspaces := filterWorkspaces(repos)

	for workspace := range workspaces {
		if err := s.exportUsers(ctx, s.logger, workspace); err != nil {
			if strings.Contains(err.Error(), "invalid status code: 403") {
				s.logger.Warn("user doesn't have access to fetch member for this workspace", "workspace", workspace)
				continue
			}
			return err
		}
	}

	return nil

}

func (s *Integration) exportRepoChildren(ctx *repoprojects.ProjectCtx, repo commonrepo.Repo) error {
	err := s.exportCommitUsersForRepo(ctx, repo)
	if err != nil {
		return err
	}

	prs, err := s.exportPullRequestsForRepo(ctx, repo)
	if err != nil {
		return err
	}

	err = s.exportGit(repo, prs)
	if err != nil {
		return err
	}

	return nil
}

func (s *Integration) exportRepos(ctx context.Context, logger hclog.Logger, sender *objsender.Session, onlyInclude []commonrepo.Repo) error {

	shouldInclude := map[string]bool{}
	for _, repo := range onlyInclude {
		shouldInclude[repo.NameWithOwner] = true
	}

	params := url.Values{}
	params.Set("pagelen", "100")
	params.Set("sort", "-updated_on")

	stopOnUpdatedAt := sender.LastProcessedTime()

	return api.Paginate(func(nextPage api.NextPage) (api.NextPage, error) {
		np, repos, err := api.ReposSourcecodePage(s.qc, params, stopOnUpdatedAt, nextPage)
		if err != nil {
			return np, err
		}

		for _, repo := range repos {
			if !shouldInclude[repo.Name] {
				continue
			}
			if err := sender.Send(repo); err != nil {
				return np, err
			}
		}
		return np, nil
	})
}

func (s *Integration) exportUsers(ctx context.Context, logger hclog.Logger, groupName string) error {

	sender, err := objsender.Root(s.agent, sourcecode.UserModelName.String())
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("pagelen", "100")

	err = api.Paginate(func(nextPage api.NextPage) (api.NextPage, error) {
		np, users, err := api.UsersSourcecodePage(s.qc, groupName, params, nextPage)
		if err != nil {
			return np, err
		}
		for _, user := range users {
			if err := sender.Send(user); err != nil {
				return np, err
			}
		}
		return np, nil
	})

	if err != nil {
		return err
	}

	return sender.Done()

}

func (s *Integration) exportCommitUsersForRepo(ctx *repoprojects.ProjectCtx, repo commonrepo.Repo) (err error) {
	usersSender, err := ctx.Session(commitusers.TableName)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("pagelen", "100")

	stopOnUpdatedAt := usersSender.LastProcessedTime()

	return api.Paginate(func(nextPage api.NextPage) (api.NextPage, error) {
		np, users, err := api.CommitUsersSourcecodePage(s.qc, ctx.Logger, repo.NameWithOwner, repo.DefaultBranch, params, stopOnUpdatedAt, nextPage)
		if err != nil {
			return np, err
		}
		for _, user := range users {
			err := user.Validate()
			if err != nil {
				s.logger.Warn("commit user", "err", err)
				continue
			}
			if err := usersSender.SendMap(user.ToMap()); err != nil {
				return np, err
			}
		}
		return np, nil
	})
}

func (s *Integration) getRepoURL(nameWithOwner string) (string, error) {

	var bbURL string
	if strings.Contains(s.config.URL, "api.bitbucket.org") {
		bbURL = strings.Replace(s.config.URL, "api.", "", -1)
	}

	u, err := url.Parse(bbURL)
	if err != nil {
		return "", err
	}
	if s.UseOAuth {
		u.User = url.UserPassword("x-token-auth", s.oauth.Get())
	} else if s.config.Username != "" {
		u.User = url.UserPassword(s.config.Username, s.config.Password)
	} else {
		return "", errors.New("no Username/Password or AccessToken passed to getRepoURL")
	}
	u.Path = nameWithOwner
	return u.String(), nil
}

func (s *Integration) exportGit(repo commonrepo.Repo, prs []rpcdef.GitRepoFetchPR) error {
	repoURL, err := s.getRepoURL(repo.NameWithOwner)
	if err != nil {
		return err
	}

	args := rpcdef.GitRepoFetch{}
	args.RepoID = s.qc.IDs.CodeRepo(repo.RefID)
	args.UniqueName = repo.NameWithOwner
	args.RefType = s.refType
	args.URL = repoURL
	args.CommitURLTemplate = commiturl.CommitURLTemplate(repo, s.config.URL)
	args.BranchURLTemplate = commiturl.BranchURLTemplate(repo, s.config.URL)
	args.PRs = prs
	if err = s.agent.ExportGitRepo(args); err != nil {
		return err
	}
	return nil
}
