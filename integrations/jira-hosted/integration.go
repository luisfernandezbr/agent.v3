package main

import (
	"context"
	"fmt"

	"github.com/pinpt/agent/pkg/reqstats"
	"github.com/pinpt/agent/pkg/structmarshal"
	pjson "github.com/pinpt/go-common/json"

	"github.com/pinpt/agent/integrations/pkg/jiracommon"
	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/objsender"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/jira-hosted/api"
	"github.com/pinpt/agent/integrations/pkg/ibase"
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
	config jiracommon.Config
	qc     api.QueryContext

	common *jiracommon.JiraCommon

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

func ConfigFromMap(data rpcdef.IntegrationConfig) (res jiracommon.Config, rerr error) {

	validationErr := func(msg string, args ...interface{}) {
		rerr = fmt.Errorf("config validation error: "+msg+"  "+pjson.Stringify(data.Config), args...)
	}

	err := structmarshal.MapToStruct(data.Config, &res)
	if err != nil {
		rerr = err
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
	s.config, err = ConfigFromMap(config.Integration)
	if err != nil {
		return err
	}

	s.qc.WebsiteURL = s.config.URL
	s.qc.CustomerID = config.Pinpoint.CustomerID
	s.qc.Logger = s.logger

	s.clientManager = reqstats.New(reqstats.Opts{
		Logger:                s.logger,
		TLSInsecureSkipVerify: true,
	})
	s.clients = s.clientManager.Clients

	{
		opts := RequesterOpts{}
		opts.Logger = s.logger
		opts.Clients = s.clients
		opts.APIURL = s.config.URL
		opts.Username = s.config.Username
		opts.Password = s.config.Password
		opts.RetryRequests = retryRequests
		requester := NewRequester(opts)
		s.qc.Req = requester
	}

	s.common, err = jiracommon.New(jiracommon.Opts{
		WebsiteURL:       s.config.URL,
		Logger:           s.logger,
		CustomerID:       config.Pinpoint.CustomerID,
		Req:              s.qc.Req,
		Agent:            s.agent,
		ExcludedProjects: s.config.Exclusions,
		IncludedProjects: s.config.Inclusions,
		Projects:         s.config.Projects,
		IsOnPremise:      true,
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

	version, err := jiracommonapi.ServerVersion(s.qc.Common())
	if err != nil {
		rerr(err)
		return
	}

	res.ServerVersion = version

	_, err = api.Projects(s.qc)
	if err != nil {
		rerr(err)
		return
	}

	return
}

type Project = jiracommon.Project

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, rerr error) {
	err := s.initWithConfig(config, true)
	if err != nil {
		rerr = err
		return
	}
	s.common.SetupUsers()

	fields, err := api.FieldsAll(s.qc)
	if err != nil {
		rerr = err
		return
	}

	fieldByID := map[string]jiracommonapi.CustomField{}
	for _, f := range fields {
		fieldByID[f.ID] = f
	}

	projectSender, err := objsender.Root(s.agent, work.ProjectModelName.String())
	if err != nil {
		rerr = err
		return
	}

	var projects []Project

	{
		allProjectsDetailed, err := api.Projects(s.qc)
		if err != nil {
			rerr = err
			return
		}

		projects, err = s.common.ProcessAllProjectsUsingExclusions(projectSender, allProjectsDetailed)
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

	issueTypesSender, err := objsender.Root(s.agent, work.IssueTypeModelName.String())
	err = s.common.IssueTypes(issueTypesSender)
	if err != nil {
		rerr = err
		return
	}
	err = issueTypesSender.Done()
	if err != nil {
		rerr = err
		return
	}

	issuePrioritiesSender, err := objsender.Root(s.agent, work.IssuePriorityModelName.String())
	err = s.common.IssuePriorities(issuePrioritiesSender)
	if err != nil {
		rerr = err
		return
	}
	err = issuePrioritiesSender.Done()
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
