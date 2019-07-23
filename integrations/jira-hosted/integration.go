package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/jira-hosted/api"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/rpcdef"
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
	s.qc.BaseURL = s.config.URL
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

	_, err = s.projects()
	if err != nil {
		return res, nil
	}

	return res, nil
}

type Project = api.Project

func (s *Integration) projects() (all []Project, _ error) {
	//sender := objsender.NewNotIncremental(s.agent, "work.project")
	//defer sender.Done()

	res, err := api.Projects(s.qc)
	if err != nil {
		return nil, err
	}

	s.logger.Info("projects returned", "projects", res)

	return nil, nil
	/*

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
			var res2 []objsender.Model
			for _, obj := range res {
				res2 = append(res2, obj)
			}
			err = sender.Send(res2)
			if err != nil {
				return false, 0, err
			}
			return pi.HasMore, pi.MaxResults, nil
		})*/
}
