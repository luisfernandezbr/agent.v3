package jiracommon

import (
	"github.com/pinpt/agent/integrations/jira/jiracommonapi"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/work"

	"github.com/hashicorp/go-hclog"
)

type Opts struct {
	WebsiteURL       string
	Logger           hclog.Logger
	CustomerID       string
	Req              jiracommonapi.Requester
	Agent            rpcdef.Agent
	ExcludedProjects []string
	IncludedProjects []string
	// Projects only process these projects by key.
	Projects    []string
	IsOnPremise bool
}

type JiraCommon struct {
	opts  Opts
	agent rpcdef.Agent

	users      *Users
	userSender *objsender.Session
}

func New(opts Opts) (*JiraCommon, error) {
	s := &JiraCommon{}
	if opts.WebsiteURL == "" || opts.CustomerID == "" || opts.Req == nil || opts.Agent == nil {
		panic("provide required params")
	}
	s.opts = opts
	s.agent = opts.Agent
	return s, nil
}

func (s *JiraCommon) SetupUsers() error {
	var err error
	s.userSender, err = objsender.Root(s.agent, work.UserModelName.String())
	if err != nil {
		return err
	}
	s.users, err = NewUsers(s.opts.Logger, s.opts.CustomerID, s.opts.Agent, s.opts.WebsiteURL, s.userSender)
	return err
}

func (s *JiraCommon) CommonQC() jiracommonapi.QueryContext {
	res := jiracommonapi.QueryContext{}
	res.WebsiteURL = s.opts.WebsiteURL
	res.CustomerID = s.opts.CustomerID
	res.Logger = s.opts.Logger
	if s.users != nil {
		res.ExportUser = s.users.ExportUser
	}
	res.Req = s.opts.Req
	res.IsOnPremise = s.opts.IsOnPremise
	res.Validate()
	return res
}

func (s *JiraCommon) ExportDone() error {
	return s.userSender.Done()
}
