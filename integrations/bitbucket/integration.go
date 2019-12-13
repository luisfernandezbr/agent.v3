package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pinpt/agent.next/integrations/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/bitbucket/api"
	"github.com/pinpt/agent.next/integrations/pkg/commiturl"
	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/pkg/commitusers"
	"github.com/pinpt/agent.next/pkg/ids2"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/agent.next/rpcdef"
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
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string

	qc api.QueryContext

	config Config

	requestConcurrencyChan chan bool

	refType string
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	s.refType = "bitbucket"

	s.qc = api.QueryContext{
		Logger: s.logger,
	}

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

	res.ApiVersion = "cloud"

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
			repoURL, err := getRepoURL(s.config.URL, url.UserPassword(s.config.Username, s.config.Password), repos[0].NameWithOwner)
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
	exportConfig rpcdef.ExportConfig) (res rpcdef.ExportResult, err error) {

	err = s.initWithConfig(exportConfig)
	if err != nil {
		return
	}

	err = s.export(ctx)

	return
}

func (s *Integration) initWithConfig(config rpcdef.ExportConfig) error {
	err := s.setIntegrationConfig(config.Integration)
	if err != nil {
		return err
	}

	s.qc.BaseURL = s.config.URL
	s.qc.CustomerID = config.Pinpoint.CustomerID
	s.qc.Logger = s.logger
	s.qc.RefType = s.refType
	s.customerID = config.Pinpoint.CustomerID

	{
		opts := api.RequesterOpts{}
		opts.Logger = s.logger
		opts.APIURL = s.config.URL + "/2.0"
		opts.Username = s.config.Username
		opts.Password = s.config.Password
		opts.InsecureSkipVerify = s.config.InsecureSkipVerify
		requester := api.NewRequester(opts)

		s.qc.Request = requester.Request
		s.qc.IDs = ids2.New(s.customerID, s.refType)
	}

	return nil
}

func (s *Integration) setIntegrationConfig(data map[string]interface{}) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}
	var def Config
	err := structmarshal.MapToStruct(data, &def)
	if err != nil {
		return err
	}
	if def.URL == "" {
		return rerr("url is missing")
	}
	if def.Username == "" {
		return rerr("username is missing")
	}
	if def.Password == "" {
		return rerr("password is missing")
	}

	s.config = def
	return nil
}

func (s *Integration) export(ctx context.Context) (err error) {

	teamSession, err := objsender.RootTracking(s.agent, "team")
	if err != nil {
		return err
	}

	teamNames, err := api.Teams(s.qc)
	if err != nil {
		return err
	}

	if err = teamSession.SetTotal(len(teamNames)); err != nil {
		return err
	}

	for _, teamName := range teamNames {
		if err := s.exportTeam(ctx, teamSession, teamName); err != nil {
			return err
		}
		if err := teamSession.IncProgress(); err != nil {
			return err
		}
	}

	return teamSession.Done()
}

func (s *Integration) exportTeam(ctx context.Context, teamSession *objsender.Session, teamName string) error {
	s.logger.Info("exporting group", "name", teamName)
	logger := s.logger.With("org", teamName)

	repos, err := commonrepo.ReposAllSlice(func(res chan []commonrepo.Repo) error {
		return api.ReposAll(s.qc, teamName, res)
	})
	if err != nil {
		return err
	}

	repos = commonrepo.Filter(logger, repos, s.config.FilterConfig)

	if s.config.OnlyGit {
		logger.Warn("only_ripsrc flag passed, skipping export of data from bitbucket api")
		for _, repo := range repos {
			err := s.exportGit(repo, nil)
			if err != nil {
				return err
			}
		}
		return nil
	}

	repoSender, err := teamSession.Session(sourcecode.RepoModelName.String(), teamName, teamName)
	if err != nil {
		return err
	}

	// export repos
	err = s.exportRepos(ctx, logger, repoSender, teamName, repos)
	if err != nil {
		return err
	}
	if err = repoSender.SetTotal(len(repos)); err != nil {
		return err
	}

	// export users
	if err = s.exportUsers(ctx, logger, teamName); err != nil {
		return err
	}

	// export repos
	if err = s.exportCommitUsers(ctx, logger, repoSender, repos); err != nil {
		return err
	}

	for _, repo := range repos {

		prSender, err := repoSender.Session(sourcecode.PullRequestModelName.String(), repo.ID, repo.NameWithOwner)
		if err != nil {
			return err
		}

		prCommitsSender, err := repoSender.Session(sourcecode.PullRequestCommitModelName.String(), repo.ID, repo.NameWithOwner)
		if err != nil {
			return err
		}

		prs, err := s.exportPullRequestsForRepo(logger, repo, prSender, prCommitsSender)
		if err != nil {
			return err
		}

		if err = s.exportGit(repo, prs); err != nil {
			return err
		}

		if err = prSender.Done(); err != nil {
			return err
		}

		if err = prCommitsSender.Done(); err != nil {
			return err
		}

	}

	return repoSender.Done()
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

func (s *Integration) exportCommitUsers(ctx context.Context, logger hclog.Logger, repoSender *objsender.Session, repos []commonrepo.Repo) (err error) {

	for _, repo := range repos {
		usersSender, err := repoSender.Session(commitusers.TableName, repo.ID, repo.NameWithOwner)
		if err != nil {
			return err
		}
		err = api.Paginate(s.logger, func(log hclog.Logger, parameters url.Values) (api.PageInfo, error) {
			pi, users, err := api.CommitUsersSourcecodePage(s.qc, repo.NameWithOwner, parameters)
			if err != nil {
				return pi, err
			}
			if err = usersSender.SetTotal(pi.Total); err != nil {
				return pi, err
			}
			for _, user := range users {
				if err := usersSender.SendMap(user.ToMap()); err != nil {
					return pi, err
				}
			}
			return pi, nil
		})
		if err != nil {
			return err
		}

		if err = usersSender.Done(); err != nil {
			return err
		}
	}

	return
}

func getRepoURL(repoURLPrefix string, user *url.Userinfo, nameWithOwner string) (string, error) {

	if strings.Contains(repoURLPrefix, "api.bitbucket.org") {
		repoURLPrefix = strings.Replace(repoURLPrefix, "api.", "", -1)
	}

	u, err := url.Parse(repoURLPrefix)
	if err != nil {
		return "", err
	}
	u.User = user
	u.Path = nameWithOwner
	return u.String(), nil
}

func (s *Integration) exportGit(repo commonrepo.Repo, prs []rpcdef.GitRepoFetchPR) error {
	urlUser := url.UserPassword(s.config.Username, s.config.Password)
	repoURL, err := getRepoURL(s.config.URL, urlUser, repo.NameWithOwner)
	if err != nil {
		return err
	}

	args := rpcdef.GitRepoFetch{}
	args.RepoID = s.qc.IDs.CodeRepo(repo.ID)
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
