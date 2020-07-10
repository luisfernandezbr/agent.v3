package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	pjson "github.com/pinpt/go-common/json"

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

	teamNames, err := api.Teams(s.qc)
	if err != nil {
		rerr(err)
		return
	}

	params := url.Values{}
	params.Set("pagelen", "1")

LOOP:
	for _, team := range teamNames {
		_, repos, err := api.ReposPage(s.qc, team, params)
		if err != nil {
			rerr(err)
			return
		}
		if len(repos) > 0 {
			repoURL, err := s.getRepoURL(repos[0].NameWithOwner)
			if err != nil {
				rerr(err)
				return
			}

			res.RepoURL = repoURL
			break LOOP
		}
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

	s.clientManager = reqstats.New(reqstats.Opts{
		Logger:                s.logger,
		TLSInsecureSkipVerify: s.config.InsecureSkipVerify,
	})

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

	teamSession, err := objsender.RootTracking(s.agent, "team")
	if err != nil {
		rerr = err
		return
	}

	teamNames, err := api.Teams(s.qc)
	if err != nil {
		rerr = err
		return
	}

	if err = teamSession.SetTotal(len(teamNames)); err != nil {
		rerr = err
		return
	}

	for _, teamName := range teamNames {
		teamResults, err := s.exportTeam(ctx, teamSession, teamName)
		if err != nil {
			rerr = err
			return
		}
		exportResults = append(exportResults, teamResults...)
		if err := teamSession.IncProgress(); err != nil {
			rerr = err
			return
		}
	}

	err = teamSession.Done()
	if err != nil {
		rerr = err
		return
	}

	s.logger.Info(s.clientManager.PrintStats())

	return
}

func (s *Integration) exportTeam(ctx context.Context, teamSession *objsender.Session, teamName string) (_ []rpcdef.ExportProject, rerr error) {
	s.logger.Info("exporting group", "name", teamName)
	logger := s.logger.With("org", teamName)

	repos, err := commonrepo.ReposAllSlice(func(res chan []commonrepo.Repo) error {
		return api.ReposAll(s.qc, teamName, res)
	})
	if err != nil {
		rerr = err
		return
	}

	repos = commonrepo.Filter(logger, repos, s.config.FilterConfig)

	if s.config.OnlyGit {
		logger.Warn("only_ripsrc flag passed, skipping export of data from bitbucket api")
		for _, repo := range repos {
			err := s.exportGit(repo, nil)
			if err != nil {
				rerr = err
				return
			}
		}
		return
	}

	repoSender, err := teamSession.Session(sourcecode.RepoModelName.String(), teamName, teamName)
	if err != nil {
		rerr = err
		return
	}

	// export repos
	err = s.exportRepos(ctx, logger, repoSender, teamName, repos)
	if err != nil {
		rerr = err
		return
	}

	s.logger.Info("exporting repos", "len", len(repos))

	if err = repoSender.SetTotal(len(repos)); err != nil {
		rerr = err
		return
	}

	// export users
	if err = s.exportUsers(ctx, logger, teamName); err != nil {
		rerr = err
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
		rerr = err
		return
	}

	err = repoSender.Done()
	if err != nil {
		rerr = err
		return
	}

	return exportResult, nil
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

func (s *Integration) exportRepos(ctx context.Context, logger hclog.Logger, sender *objsender.Session, groupName string, onlyInclude []commonrepo.Repo) error {

	shouldInclude := map[string]bool{}
	for _, repo := range onlyInclude {
		shouldInclude[repo.NameWithOwner] = true
	}

	return api.PaginateNewerThan(s.logger, sender.LastProcessedTime(), func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, repos, err := api.ReposSourcecodePage(s.qc, groupName, parameters, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}

		if err = sender.SetTotal(pi.Total); err != nil {
			return pi, err
		}

		for _, repo := range repos {
			if !shouldInclude[repo.Name] {
				continue
			}
			if err := sender.Send(repo); err != nil {
				return pi, err
			}
		}
		return pi, nil
	})
}

func (s *Integration) exportUsers(ctx context.Context, logger hclog.Logger, groupName string) error {

	sender, err := objsender.Root(s.agent, sourcecode.UserModelName.String())
	if err != nil {
		return err
	}

	err = api.Paginate(s.logger, func(log hclog.Logger, parameters url.Values) (api.PageInfo, error) {
		pi, users, err := api.UsersSourcecodePage(s.qc, groupName, parameters)
		if err != nil {
			return pi, err
		}
		if err = sender.SetTotal(pi.Total); err != nil {
			return pi, err
		}
		for _, user := range users {
			if err := sender.Send(user); err != nil {
				return pi, err
			}
		}
		return pi, nil
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
	return api.PaginateNewerThan(ctx.Logger, usersSender.LastProcessedTime(), func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, users, err := api.CommitUsersSourcecodePage(s.qc, repo.NameWithOwner, parameters, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		if err = usersSender.SetTotal(pi.Total); err != nil {
			return pi, err
		}
		for _, user := range users {
			err := user.Validate()
			if err != nil {
				s.logger.Warn("commit user", "err", err)
				continue
			}
			if err := usersSender.SendMap(user.ToMap()); err != nil {
				return pi, err
			}
		}
		return pi, nil
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
