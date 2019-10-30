package main

import (
	"context"
	"fmt"

	"github.com/pinpt/agent.next/pkg/reqstats"
	"github.com/pinpt/agent.next/pkg/structmarshal"

	"github.com/pinpt/agent.next/integrations/pkg/jiracommon"
	"github.com/pinpt/agent.next/integrations/pkg/objsender"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/jira-hosted/api"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/rpcdef"
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

type Config jiracommon.Config

func ConfigFromMap(data map[string]interface{}) (res Config, rerr error) {
	validationErr := func(msg string, args ...interface{}) {
		rerr = fmt.Errorf("config validation error: "+msg, args...)
	}
	err := structmarshal.MapToStruct(data, &res)
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

func (s *Integration) initWithConfig(config rpcdef.ExportConfig) error {
	var err error
	s.config, err = ConfigFromMap(config.Integration)
	if err != nil {
		return err
	}

	s.qc.BaseURL = s.config.URL
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
		requester := NewRequester(opts)
		s.qc.Request = requester.Request
	}

	s.common, err = jiracommon.New(jiracommon.Opts{
		BaseURL:          s.config.URL,
		Logger:           s.logger,
		CustomerID:       config.Pinpoint.CustomerID,
		Request:          s.qc.Request,
		Agent:            s.agent,
		ExcludedProjects: s.config.ExcludedProjects,
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

	err := s.initWithConfig(exportConfig)
	if err != nil {
		rerr(err)
		return
	}

	_, err = api.Projects(s.qc, true)
	if err != nil {
		rerr(err)
		return
	}

	return
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, rerr error) {
	err := s.initWithConfig(config)
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
		allProjectsDetailed, err := api.Projects(s.qc, false)
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

	err = s.common.IssuesAndChangelogs(projectSender, projects, fieldByID)
	if err != nil {
		rerr = err
		return
	}

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
		err := sender.Send(item)
		if err != nil {
			rerr = err
			return
		}
	}

	return res, sender.Done()
}
