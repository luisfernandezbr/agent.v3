package main

import (
	"context"
	"errors"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/integrations/gcal/api"
	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/pkg/oauthtoken"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/calendar"
)

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}

// IntegrationConfig _
type IntegrationConfig struct {
	Exclusions []string `json:"exclusions"`
	Inclusions []string `json:"inclusions"`

	AccessToken string `json:"access_token"`
	Local       bool   `json:"local"`
}

// Integration _
type Integration struct {
	logger  hclog.Logger
	agent   rpcdef.Agent
	refType string
	config  IntegrationConfig
}

// NewIntegration _
func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}

// Init _
func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	s.refType = "gcal"
	return nil
}

// Calendar used in repoprojects.ProcessOpts
type Calendar struct {
	RefID string
	Name  string
	API   api.API
}

// GetID gets the ref id
func (s Calendar) GetID() string {
	return s.RefID
}

// GetReadableID gets the name
func (s Calendar) GetReadableID() string {
	return s.Name
}

func containsRefID(list []string, one string) bool {
	if list == nil {
		return false
	}
	for _, each := range list {
		if each == one {
			return true
		}
	}
	return false
}

// Export exports all the calendars in the Inclusions list and its events
func (s *Integration) Export(ctx context.Context, conf rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	s.logger.Info("starting gcal export")
	session, err := objsender.Root(s.agent, calendar.CalendarModelName.String())
	if err != nil {
		s.logger.Error("error creating calendar session", "err", err)
		return res, err
	}
	defer session.Done()
	session.SetNoAutoProgress(true)
	var projectsIface []repoprojects.RepoProject
	if len(s.config.Inclusions) > 0 {
		for _, refreshToken := range s.config.Inclusions {

			api, err := api.New(s.logger, conf.Pinpoint.CustomerID, s.refType, func() (string, error) {
				return s.agent.OAuthNewAccessTokenFromRefreshToken(s.refType, refreshToken)
			})

			cals, err := api.GetCalendars()
			if err != nil {
				s.logger.Warn("error fetching calendars for user, skipping", "err", err, "refreshToken", refreshToken)
				continue
			}
			for _, cal := range cals {
				if err := session.Send(cal); err != nil {
					s.logger.Error("error sending event to agent", "err", err, "id", cal.RefID)
					return res, err
				}
				projectsIface = append(projectsIface, Calendar{
					RefID: cal.Description, // this is the email, use this to get events
					Name:  cal.Name,
					API:   api,
				})
			}
		}
	} else {
		api, err := s.initAPI(conf)
		if err != nil {
			return res, err
		}
		cals, err := api.GetCalendars()
		if err != nil {
			s.logger.Error("error fetching calendars", "err", err)
			return res, err
		}
		for _, cal := range cals {
			if containsRefID(s.config.Exclusions, cal.RefID) {
				continue
			}
			if err := session.Send(cal); err != nil {
				s.logger.Error("error sending event to agent", "err", err, "id", cal.RefID)
				return res, err
			}
			projectsIface = append(projectsIface, Calendar{
				RefID: cal.Description, // this is the email, use this to get events
				Name:  cal.Name,
				API:   api,
			})
		}
	}

	userchan := make(chan map[string]*calendar.User, len(projectsIface))

	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.logger
	processOpts.ProjectLastProcessFn = func(ctx *repoprojects.ProjectCtx) (string, error) {
		proj := ctx.Project.(Calendar)
		eventSender, err := ctx.Session(calendar.EventModelName)
		if err != nil {
			return "", err
		}
		s.logger.Info("trying to get events for user_id", "user_id", proj.RefID, "name", proj.Name)
		events, users, nextToken, err := proj.API.GetEventsAndUsers(proj.RefID, eventSender.LastProcessed())
		if err != nil {
			s.logger.Error("error fetching events for user_id, skipping", "err", err, "user_id", proj.RefID, "name", proj.Name)
			return "", err
		}
		for _, evt := range events {
			if err := eventSender.Send(evt); err != nil {
				return "", err
			}
		}

		userchan <- users
		return nextToken, err
	}
	rerr := make(chan error, 1)
	go func() {
		allusers := make(map[string]*calendar.User)
		for usrs := range userchan {
			for k, v := range usrs {
				allusers[k] = v
			}
		}
		userSender, err := objsender.Root(s.agent, calendar.UserModelName.String())
		if err != nil {
			rerr <- err
			return
		}
		defer userSender.Done()
		for _, user := range allusers {
			if err := userSender.Send(user); err != nil {
				rerr <- err
				return
			}
		}
		rerr <- nil
	}()
	processOpts.Concurrency = 10
	processOpts.Projects = projectsIface
	processOpts.IntegrationType = inconfig.IntegrationTypeCalendar
	processOpts.CustomerID = conf.Pinpoint.CustomerID
	processOpts.RefType = s.refType
	processOpts.Sender = session

	processor := repoprojects.NewProcess(processOpts)
	res.Projects, err = processor.Run()
	if err != nil {
		return res, err
	}
	close(userchan)
	err = <-rerr
	return
}

// ValidateConfig calls a simple api to make sure we have the correct credentials
func (s *Integration) ValidateConfig(ctx context.Context, conf rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {

	api, err := s.initAPI(conf)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}
	if err := api.Validate(); err != nil {
		res.Errors = append(res.Errors, err.Error())
		s.logger.Info("error with validate", "err", err)
		return res, err
	}
	return
}

// OnboardExport returns the data used in onboard
func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, conf rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {

	api, err := s.initAPI(conf)
	if err != nil {
		res.Error = err
		return res, err
	}
	cals, err := api.GetCalendars()
	if err != nil {
		res.Error = err
		return res, err
	}
	var records []map[string]interface{}
	for _, c := range cals {
		calres := agent.CalendarResponseCalendars{
			Description: c.Description,
			Name:        c.Name,
			RefID:       c.RefID,
			RefType:     c.RefType,
			Active:      true,
			Enabled:     true,
		}
		records = append(records, calres.ToMap())
	}
	res.Data = records
	return
}

// Mutate changes integration data
func (s *Integration) Mutate(ctx context.Context, fn string, data string, conf rpcdef.ExportConfig) (res rpcdef.MutateResult, _ error) {
	return res, errors.New("mutate not supported")
}

func (s *Integration) initAPI(conf rpcdef.ExportConfig) (api.API, error) {
	if err := structmarshal.MapToStruct(conf.Integration.Config, &s.config); err != nil {
		s.logger.Error("error creating the config object", "err", err)
		return nil, err
	}

	return api.New(s.logger, conf.Pinpoint.CustomerID, s.refType, func() (string, error) {
		if s.config.Local {
			if s.config.AccessToken == "" {
				return "", errors.New("access token required")
			}
			return s.config.AccessToken, nil
		}
		oauth, err := oauthtoken.New(s.logger, s.agent)
		if err != nil {
			return "", err
		}
		return oauth.Get(), nil
	})
}
