package main

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/jira/api"
	"github.com/pinpt/agent.next/rpcdef"
)

type Integration struct {
	logger hclog.Logger
	agent  rpcdef.Agent
	config Config

	qc api.QueryContext
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
	URL      string
	Username string
	Password string
}

func (s *Integration) setIntegrationConfig(data map[string]interface{}) error {
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}
	conf := Config{}

	conf.URL, _ = data["url"].(string)
	if conf.URL == "" {
		return rerr("url is missing")
	}

	conf.Username, _ = data["username"].(string)
	if conf.Username == "" {
		return rerr("username is missing")
	}

	conf.Password, _ = data["password"].(string)
	if conf.Password == "" {
		return rerr("password is missing")
	}

	s.config = conf
	return nil
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	err := s.setIntegrationConfig(config.Integration)
	if err != nil {
		return res, err
	}

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

	projects, err := s.projects()
	if err != nil {
		return res, err
	}

	for _, project := range projects {
		err := s.issues(project)
		if err != nil {
			return res, err
		}
		break
	}

	return res, nil
}

const apiVersion = "3"

type Project = api.Project

func (s *Integration) projects() (all []Project, _ error) {
	sender := newSenderNoIncrementals(s, "work.project")
	defer sender.Done()

	return all, api.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, _ error) {
		pi, res, err := api.ProjectsPage(s.qc, paginationParams)
		if err != nil {
			return false, 0, err
		}
		for _, obj := range res {
			p := Project{}
			p.JiraID = obj.RefID
			p.Key = obj.Identifier
			all = append(all, p)
		}
		var res2 []Model
		for _, obj := range res {
			res2 = append(res2, obj)
		}
		err = sender.Send(res2)
		if err != nil {
			return false, 0, err
		}
		return pi.HasMore, pi.MaxResults, nil
	})
}

func (s *Integration) issues(project Project) error {
	sender := newSenderNoIncrementals(s, "work.issue")
	defer sender.Done()

	return api.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, _ error) {
		pi, res, err := api.IssuesPage(s.qc, project, paginationParams)
		if err != nil {
			return false, 0, err
		}
		var res2 []Model
		for _, obj := range res {
			res2 = append(res2, obj)
		}
		err = sender.Send(res2)
		if err != nil {
			return false, 0, err
		}
		return pi.HasMore, pi.MaxResults, nil
	})
}

type senderNoIncrementals struct {
	RefType     string
	SessionID   string
	integration *Integration
}

func newSenderNoIncrementals(integration *Integration, refType string) *senderNoIncrementals {
	s := &senderNoIncrementals{}
	s.RefType = refType
	s.integration = integration
	s.SessionID, _ = s.integration.agent.ExportStarted(s.RefType)
	return s
}

type Model interface {
	ToMap(...bool) map[string]interface{}
}

func (s *senderNoIncrementals) Send(objs []Model) error {
	if len(objs) == 0 {
		return nil
	}
	var objs2 []rpcdef.ExportObj
	for _, obj := range objs {
		objs2 = append(objs2, rpcdef.ExportObj{Data: obj.ToMap()})
	}
	s.integration.agent.SendExported(s.SessionID, objs2)
	return nil
}

func (s *senderNoIncrementals) Done() {
	s.integration.agent.ExportDone(s.SessionID, nil)
}
