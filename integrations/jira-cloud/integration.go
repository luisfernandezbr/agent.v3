package main

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/pkg/structmarshal"

	"github.com/pinpt/agent.next/pkg/objsender"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/jira-cloud/api"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/integrations/pkg/jiracommon"
	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
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
	URL              string   `json:"url"`
	Username         string   `json:"username"`
	Password         string   `json:"password"`
	ExcludedProjects []string `json:"excluded_projects"`
}

func (s *Integration) setIntegrationConfig(data map[string]interface{}) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}
	var conf Config
	err := structmarshal.MapToStruct(data, &conf)
	if err != nil {
		return err
	}
	if conf.URL == "" {
		return rerr("url is missing")
	}
	if conf.Username == "" {
		return rerr("username is missing")
	}
	if conf.Password == "" {
		return rerr("password is missing")
	}
	s.config = conf
	return nil
}

func (s *Integration) initWithConfig(config rpcdef.ExportConfig) error {
	err := s.setIntegrationConfig(config.Integration)
	if err != nil {
		return err
	}

	s.qc.BaseURL = s.config.URL
	s.qc.CustomerID = config.Pinpoint.CustomerID
	s.qc.Logger = s.logger

	{
		opts := api.RequesterOpts{}
		opts.Logger = s.logger
		opts.APIURL = s.config.URL
		opts.Username = s.config.Username
		opts.Password = s.config.Password
		requester := api.NewRequester(opts)

		s.qc.Request = requester.Request
	}

	s.common, err = jiracommon.New(jiracommon.Opts{
		BaseURL:          s.config.URL,
		Logger:           s.logger,
		CustomerID:       config.Pinpoint.CustomerID,
		Request:          s.qc.Request,
		Agent:            s.agent,
		ExcludedProjects: s.config.ExcludedProjects,
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

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {

	err := s.initWithConfig(config)
	if err != nil {
		return res, err
	}

	s.common.SetupUsers()
	defer s.common.ExportDone()

	fields, err := s.fields()
	if err != nil {
		return res, err
	}

	fieldByID := map[string]*work.CustomField{}
	for _, f := range fields {
		fieldByID[f.RefID] = f
	}

	var projects []Project
	{
		allProjectsDetailed, err := s.projects()
		if err != nil {
			return res, err
		}

		projects, err = s.common.ProcessAllProjectsUsingExclusions(allProjectsDetailed)
		if err != nil {
			return res, err
		}
	}

	err = s.common.IssuesAndChangelogs(projects, fieldByID)
	if err != nil {
		return res, err
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

func (s *Integration) fields() ([]*work.CustomField, error) {
	sender := objsender.NewNotIncremental(s.agent, "work.custom_field")
	defer sender.Done()

	res, err := api.FieldsAll(s.qc)
	if err != nil {
		return nil, err
	}
	var res2 []objsender.Model
	for _, item := range res {
		res2 = append(res2, item)
	}
	return res, sender.Send(res2)
}
