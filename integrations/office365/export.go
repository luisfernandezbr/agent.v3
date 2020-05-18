package main

import (
	"context"
	"errors"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/integrations/office365/api"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/calendar"
)

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
func (s *Integration) export(ctx context.Context, conf rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	if err := s.initAPI(conf); err != nil {
		return res, err
	}

	if len(s.config.Inclusions) == 0 {
		return res, errors.New("no inclusions")
	}
	session, err := objsender.Root(s.agent, calendar.CalendarModelName.String())
	if err != nil {
		s.logger.Error("error creating calendar session", "err", err)
		return res, err
	}
	defer session.Done()
	session.SetNoAutoProgress(true)

	var projectsIface []repoprojects.RepoProject
	for _, refreshToken := range s.config.Inclusions {
		api, err := api.New(s.logger, conf.Pinpoint.CustomerID, s.refType, func() (string, error) {
			return s.agent.OAuthNewAccessTokenFromRefreshToken("office365", refreshToken)
		})
		if err != nil {
			s.logger.Error("something wrong with refresh token", "err", err)
			return res, err
		}
		cals, err := api.GetMainCalendars()
		if err != nil {
			s.logger.Error("error fetching calendar, skipping", "err", err)
			continue
		}
		for _, cal := range cals {
			if err := session.Send(cal); err != nil {
				s.logger.Error("error sending event to agent", "err", err, "id", cal.RefID)
				return res, err
			}
			projectsIface = append(projectsIface, Calendar{
				RefID: cal.RefID,
				Name:  cal.Name,
				API:   api,
			})
		}
	}

	userchan := make(chan map[string]*calendar.User, len(projectsIface))

	processOpts := repoprojects.ProcessOpts{}
	processOpts.Logger = s.logger
	processOpts.ProjectFn = func(ctx *repoprojects.ProjectCtx) error {
		proj := ctx.Project.(Calendar)
		eventSender, err := ctx.Session(calendar.EventModelName)
		if err != nil {
			return err
		}
		events, users, err := proj.API.GetEventsAndUsers(proj.GetID())
		if err != nil {
			return err
		}
		for _, evt := range events {
			if err := eventSender.Send(evt); err != nil {
				return err
			}
		}
		userchan <- users
		return nil
	}
	processOpts.Concurrency = 10
	processOpts.Projects = projectsIface
	processOpts.IntegrationType = inconfig.IntegrationTypeCalendar
	processOpts.CustomerID = conf.Pinpoint.CustomerID
	processOpts.RefType = s.refType
	processOpts.Sender = session

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

	processor := repoprojects.NewProcess(processOpts)
	res.Projects, err = processor.Run()

	if err != nil {
		return res, err
	}
	close(userchan)
	err = <-rerr
	return

}
