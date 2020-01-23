package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/pinpt/agent/pkg/oauthtoken"
	"github.com/pinpt/agent/pkg/reqstats"
	"github.com/pinpt/agent/pkg/structmarshal"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/jira-cloud/api"
	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/integrations/pkg/jiracommon"
	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/work"
)

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}

type Integration struct {
	logger hclog.Logger
	agent  rpcdef.Agent
	config Config
	qc     api.QueryContext

	common *jiracommon.JiraCommon

	UseOAuth      bool
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
	return nil
}

type Config struct {
	jiracommon.Config
	RefreshToken string `json:"refresh_token"`
}

func setConfig(config rpcdef.ExportConfig) (res Config, rerr error) {
	data := config.Integration
	validationErr := func(msg string, args ...interface{}) {
		rerr = fmt.Errorf("config validation error: "+msg, args...)
	}
	err := structmarshal.MapToStruct(data, &res)
	if err != nil {
		rerr = err
		return
	}
	if config.UseOAuth {
		// no required fields for OAuth
		return
	}
	if res.URL == "" {
		validationErr("url is missing")
		return
	}
	if res.Username == "" {
		validationErr("username is missing")
		return
	}
	if res.Password == "" {
		validationErr("password is missing")
		return
	}
	return
}

func (s *Integration) initWithConfig(config rpcdef.ExportConfig, retryRequests bool) error {
	var err error
	s.config, err = setConfig(config)
	if err != nil {
		return err
	}
	s.UseOAuth = config.UseOAuth

	s.clientManager = reqstats.New(reqstats.Opts{
		Logger:                s.logger,
		TLSInsecureSkipVerify: false,
	})
	s.clients = s.clientManager.Clients

	var oauth *oauthtoken.Manager

	apiBaseURL := ""
	s.qc.WebsiteURL = s.config.URL

	if s.UseOAuth {
		oauth, err = oauthtoken.New(s.logger, s.agent)
		if err != nil {
			return err
		}

		sites, err := api.AccessibleResources(s.logger, s.clients, oauth.Get())
		if err != nil {
			return err
		}
		if len(sites) == 0 {
			return errors.New("no accessible-resources resources found for oauth token")
		}
		if len(sites) > 1 {
			return errors.New("more than 1 site accessible with oauth token, this is not supported")
		}
		site := sites[0]
		apiBaseURL = "https://api.atlassian.com/ex/jira/" + site.ID

	} else {
		apiBaseURL = s.config.URL
	}

	{
		opts := RequesterOpts{}
		opts.Logger = s.logger
		opts.APIURL = apiBaseURL
		opts.Clients = s.clients
		opts.RetryRequests = retryRequests

		if s.UseOAuth {
			opts.OAuthToken = oauth
		} else {
			opts.Username = s.config.Username
			opts.Password = s.config.Password
		}
		requester := NewRequester(opts)
		s.qc.Request = requester.Request
		s.qc.Request2 = requester.Request2
	}

	s.qc.CustomerID = config.Pinpoint.CustomerID
	s.qc.Logger = s.logger

	s.common, err = jiracommon.New(jiracommon.Opts{
		WebsiteURL:       s.qc.WebsiteURL,
		Logger:           s.logger,
		CustomerID:       config.Pinpoint.CustomerID,
		Request:          s.qc.Request,
		Agent:            s.agent,
		ExcludedProjects: s.config.ExcludedProjects,
		IncludedProjects: s.config.IncludedProjects,
		Projects:         s.config.Projects,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Integration) ValidateConfig(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {

	rerr := func(err error) {
		res.Errors = append(res.Errors, err.Error())
	}

	err := s.initWithConfig(exportConfig, false)
	if err != nil {
		rerr(err)
		return
	}

	err = jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, _ error) {
		_, _, err := api.ProjectsPage(s.qc, paginationParams)
		if err != nil {
			return false, 10, err
		}
		return false, 10, nil
	})
	if err != nil {
		rerr(err)
		return
	}

	return
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, rerr error) {

	err := s.initWithConfig(config, true)
	if err != nil {
		rerr = err
		return
	}

	s.common.SetupUsers()

	fields, err := s.fields()
	if err != nil {
		rerr = err
		return
	}

	fieldByID := map[string]*work.CustomField{}
	for _, f := range fields {
		fieldByID[f.RefID] = f
	}

	projectSender, err := objsender.Root(s.agent, work.ProjectModelName.String())
	if err != nil {
		rerr = err
		return
	}

	var projects []Project

	{
		allProjectsDetailed, err := s.projects()
		if err != nil {
			rerr = err
			return
		}

		projects, err = s.common.ProcessAllProjectsUsingExclusions(projectSender, allProjectsDetailed)
		if err != nil {
			rerr = err
			return
		}

		err = projectSender.SetTotal(len(projects))
		if err != nil {
			rerr = err
			return
		}
	}

	exportProjectResults, err := s.common.IssuesAndChangelogs(projectSender, projects, fieldByID)
	if err != nil {
		rerr = err
		return
	}

	res.Projects = exportProjectResults

	err = projectSender.Done()
	if err != nil {
		rerr = err
		return
	}

	err = s.common.ExportDone()
	if err != nil {
		rerr = err
		return
	}

	return res, nil
}

type Project = jiracommon.Project

func (s *Integration) projects() (all []*work.Project, _ error) {
	return all, jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, _ error) {
		pi, res, err := api.ProjectsPage(s.qc, paginationParams)
		if err != nil {
			return false, 0, err
		}
		for _, obj := range res {
			all = append(all, obj)
		}

		return pi.HasMore, pi.MaxResults, nil
	})
}

func (s *Integration) fields() (_ []*work.CustomField, rerr error) {
	sender, err := objsender.Root(s.agent, work.CustomFieldModelName.String())
	if err != nil {
		rerr = err
		return
	}
	res, err := api.FieldsAll(s.qc)
	if err != nil {
		rerr = err
		return
	}
	for _, item := range res {
		err = sender.Send(item)
		if err != nil {
			rerr = err
			return
		}
	}
	return res, sender.Done()
}
