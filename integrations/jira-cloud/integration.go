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
	pjson "github.com/pinpt/go-common/json"
	"github.com/pinpt/integration-sdk/work"

	pstrings "github.com/pinpt/go-common/strings"
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

func setConfig(config rpcdef.ExportConfig) (res jiracommon.Config, rerr error) {

	data := config.Integration
	validationErr := func(msg string, args ...interface{}) {
		rerr = fmt.Errorf("config validation error: "+msg+"  "+pjson.Stringify(config.Integration.Config), args...)
	}

	err := structmarshal.MapToStruct(data.Config, &res)
	if err != nil {
		rerr = err
		return
	}

	if res.URL == "" {
		validationErr("url is missing")

	}
	if config.UseOAuth {
		// no required fields for OAuth
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
		var site api.Site
		for _, item := range sites {
			if item.URL == s.qc.WebsiteURL {
				site = item
			}
		}
		if site.URL == "" {
			var authed []string
			for _, item := range sites {
				authed = append(authed, item.URL)
			}
			return fmt.Errorf("This account is not authorized for jira with the following url: %v, it can only access the following instances: %v", s.qc.WebsiteURL, authed)
		}
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
		s.qc.Req = requester
	}

	s.qc.CustomerID = config.Pinpoint.CustomerID
	s.qc.Logger = s.logger

	s.common, err = jiracommon.New(jiracommon.Opts{
		WebsiteURL:       s.qc.WebsiteURL,
		Logger:           s.logger,
		CustomerID:       config.Pinpoint.CustomerID,
		Req:              s.qc.Req,
		Agent:            s.agent,
		ExcludedProjects: s.config.Exclusions,
		IncludedProjects: s.config.Inclusions,
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
	projectMap := make(map[string]string)
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

		projectMapAll := make(map[string]string)
		for _, project := range allProjectsDetailed {
			projectMapAll[project.RefID] = project.ID
		}

		for _, prj := range projects {
			if prjID, ok := projectMapAll[prj.JiraID]; ok {
				projectMap[prj.JiraID] = prjID
			}
		}

	}

	issueStatusSender, err := objsender.Root(s.agent, work.IssueStatusModelName.String())
	issueStatus, err := s.issueStatus(issueStatusSender)
	if err != nil {
		rerr = err
		return
	}
	err = issueStatusSender.Done()
	if err != nil {
		rerr = err
		return
	}

	s.common.SetIssueStatus(issueStatus)

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

	if err = s.boards(issueStatus, projectMap); err != nil {
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

func (s *Integration) issueStatus(sender *objsender.Session) (res map[string]*work.IssueStatus, _ error) {
	s.logger.Debug("exporting issue statuses")

	res = make(map[string]*work.IssueStatus)

	statuses, _, err := jiracommonapi.StatusWithDetail(s.common.CommonQC())
	if err != nil {
		return res, err
	}

	if err != nil {
		return res, err
	}
	for _, item := range statuses {

		issueStatus := &work.IssueStatus{
			CustomerID:  s.qc.CustomerID,
			Description: item.Description,
			IconURL:     pstrings.Pointer(item.IconURL),
			Name:        item.Name,
			RefID:       item.ID,
			RefType:     "jira",
		}

		res[item.ID] = issueStatus

		err := sender.Send(issueStatus)
		if err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *Integration) boards(issueStatus map[string]*work.IssueStatus, projectMap map[string]string) error {
	boardsSender, err := objsender.Root(s.agent, work.KanbanBoardModelName.String())
	if err != nil {
		return err
	}

	boards, err := s.allBoards()
	if err != nil {
		return err
	}

	for _, board := range boards {
		projectRefIDs, err := s.allProjectsList(board.RefID)
		if err != nil {
			return err
		}

		if !projectsExistOnMap(projectMap, projectRefIDs) {
			continue
		}

		board.ProjectIds = convertProjectRefIDsToIDs(projectMap, projectRefIDs)

		boardColumns, err := api.BoardColumnsStatuses(s.qc, issueStatus, board.RefID)
		if err != nil {
			return err
		}

		board.Columns = boardColumns

		err = boardsSender.Send(board)
		if err != nil {
			return err
		}
	}

	if err := boardsSender.Done(); err != nil {
		return err
	}

	return nil
}

func projectsExistOnMap(projectMap map[string]string, refIDs []string) (exist bool) {
	exist = true
	for _, refID := range refIDs {
		if _, ok := projectMap[refID]; !ok {
			return false
		}
	}
	return
}

func convertProjectRefIDsToIDs(projectMap map[string]string, refIDs []string) (ids []string) {
	for _, refID := range refIDs {
		ids = append(ids, projectMap[refID])
	}

	return
}

func (s *Integration) allBoards() (all []*work.KanbanBoard, _ error) {
	return all, jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, _ error) {
		pi, res, err := api.BoardsPage(s.qc, paginationParams)
		if err != nil {
			return false, 0, err
		}
		for _, obj := range res {
			all = append(all, obj)
		}

		return pi.HasMore, pi.MaxResults, nil
	})
}

func (s *Integration) allProjectsList(boardID string) (all []string, _ error) {
	return all, jiracommonapi.PaginateStartAt(func(paginationParams url.Values) (hasMore bool, pageSize int, _ error) {
		pi, res, err := api.BoardProjectListPage(boardID, s.qc, paginationParams)
		if err != nil {
			return false, 0, err
		}
		for _, obj := range res {
			all = append(all, obj)
		}

		return pi.HasMore, pi.MaxResults, nil
	})
}
