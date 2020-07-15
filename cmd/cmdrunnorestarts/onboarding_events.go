package cmdrunnorestarts

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/pinpt/agent/cmd/cmdexportonboarddata"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/subcommand"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/go-common/v10/datamodel"
	"github.com/pinpt/go-common/v10/event/action"
	"github.com/pinpt/go-common/v10/eventing"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *runner) handleOnboardingEvents(ctx context.Context) (closefunc, error) {
	s.logger.Info("listening for onboarding requests")

	processOnboard := func(msg eventing.Message, integration map[string]interface{}, systemType inconfig.IntegrationType, objectType string) (data cmdexportonboarddata.Result, rerr error) {
		atomic.AddInt64(&s.onboardingInProgress, 1)
		defer func() {
			atomic.AddInt64(&s.onboardingInProgress, -1)
		}()

		s.logger.Info("received onboard request", "type", objectType)
		header, err := parseHeader(msg.Headers)
		if err != nil {
			return data, fmt.Errorf("error parsing header. err %v", err)
		}

		ctx := context.Background()
		conf, err := inconfig.AuthFromEvent(integration, s.conf.PPEncryptionKey)
		conf.Type = systemType
		if err != nil {
			rerr = err
			return
		}

		data, err = s.getOnboardData(ctx, conf, header.MessageID, objectType)
		if err != nil {
			rerr = err
			return
		}

		return data, nil
	}

	cbRepo := func(instance datamodel.ModelReceiveEvent) (_ datamodel.ModelSendEvent, _ error) {

		rerr := func(err error) {
			s.logger.Error("could not process repo requests event", "err", err)
		}

		req := instance.Object().(*agent.RepoRequest)
		data, err := processOnboard(instance.Message(), req.Integration.ToMap(), inconfig.IntegrationType(req.Integration.SystemType), "repos")

		if err != nil {
			rerr(err)
			return
		}

		resp := &agent.RepoResponse{}
		resp.Type = agent.RepoResponseTypeRepo
		resp.RefType = req.RefType
		resp.RefID = req.RefID
		resp.RequestID = req.ID
		resp.IntegrationID = req.Integration.ID

		resp.Success = data.Success
		if data.Error != "" {
			resp.Error = pstrings.Pointer(data.Error)
		}

		if data.Data != nil {
			var records cmdexportonboarddata.DataRepos
			err := structmarshal.AnyToAny(data.Data, &records)
			if err != nil {
				rerr(fmt.Errorf("invalid data format returned in agent onboard: %v", err))
			}
			for _, rec := range records {
				repo := &agent.RepoResponseRepos{}
				repo.FromMap(rec)
				resp.Repos = append(resp.Repos, *repo)
			}
		}

		s.deviceInfo.AppendCommonInfo(resp)
		return datamodel.NewModelSendEvent(resp), nil
	}

	cbCalendar := func(instance datamodel.ModelReceiveEvent) (_ datamodel.ModelSendEvent, _ error) {

		rerr := func(err error) {
			s.logger.Error("could not process calendar requests event", "err", err)
		}

		req := instance.Object().(*agent.CalendarRequest)
		data, err := processOnboard(instance.Message(), req.Integration.ToMap(), inconfig.IntegrationType(req.Integration.SystemType), "calendars")

		if err != nil {
			rerr(err)
			return
		}

		resp := &agent.CalendarResponse{}
		resp.Type = agent.CalendarResponseTypeStop
		resp.RefType = req.RefType
		resp.RefID = req.RefID
		resp.RequestID = req.ID
		resp.IntegrationID = req.Integration.ID

		resp.Success = data.Success
		if data.Error != "" {
			resp.Error = pstrings.Pointer(data.Error)
		}

		if data.Data != nil {
			var records cmdexportonboarddata.DataCalendars
			err := structmarshal.AnyToAny(data.Data, &records)
			if err != nil {
				rerr(fmt.Errorf("invalid data format returned in agent onboard: %v", err))
			}
			for _, rec := range records {
				cal := &agent.CalendarResponseCalendars{}
				cal.FromMap(rec)
				resp.Calendars = append(resp.Calendars, *cal)
			}
		}

		s.deviceInfo.AppendCommonInfo(resp)
		return datamodel.NewModelSendEvent(resp), nil
	}

	cbProject := func(instance datamodel.ModelReceiveEvent) (_ datamodel.ModelSendEvent, _ error) {

		rerr := func(err error) {
			s.logger.Error("could not process project requests event", "err", err)
		}

		req := instance.Object().(*agent.ProjectRequest)
		data, err := processOnboard(instance.Message(), req.Integration.ToMap(), inconfig.IntegrationType(req.Integration.SystemType), "projects")
		if err != nil {
			rerr(err)
			return
		}

		resp := &agent.ProjectResponse{}
		resp.Type = agent.ProjectResponseTypeProject
		resp.RefType = req.RefType
		resp.RefID = req.RefID
		resp.RequestID = req.ID
		resp.IntegrationID = req.Integration.ID

		resp.Success = data.Success
		if data.Error != "" {
			resp.Error = pstrings.Pointer(data.Error)
		}

		if data.Data != nil {
			var records cmdexportonboarddata.DataProjects
			err := structmarshal.AnyToAny(data.Data, &records)
			if err != nil {
				rerr(fmt.Errorf("invalid data format returned in agent onboard: %v", err))
			}
			for _, rec := range records {
				project := &agent.ProjectResponseProjects{}
				project.FromMap(rec)
				resp.Projects = append(resp.Projects, *project)
			}
		}
		s.deviceInfo.AppendCommonInfo(resp)
		return datamodel.NewModelSendEvent(resp), nil
	}

	cbWorkconfig := func(instance datamodel.ModelReceiveEvent) (_ datamodel.ModelSendEvent, _ error) {

		rerr := func(err error) {
			s.logger.Error("could not process work config event", "err", err)
		}

		req := instance.Object().(*agent.WorkStatusRequest)
		data, err := processOnboard(instance.Message(), req.Integration.ToMap(), inconfig.IntegrationType(req.Integration.SystemType), "workconfig")
		if err != nil {
			rerr(err)
			return
		}

		resp := &agent.WorkStatusResponse{}
		resp.Type = agent.WorkStatusResponseTypeProject
		resp.RefType = req.RefType
		resp.RefID = req.RefID
		resp.RequestID = req.ID
		resp.IntegrationID = req.Integration.ID

		resp.Success = data.Success
		if data.Error != "" {
			resp.Error = pstrings.Pointer(data.Error)
		}

		workStatuses := &agent.WorkStatusResponseWorkConfig{}
		if data.Data != nil {
			var m cmdexportonboarddata.DataWorkConfig
			err := structmarshal.AnyToAny(data.Data, &m)
			if err != nil {
				rerr(fmt.Errorf("invalid data format returned in agent onboard: %v", err))
			}
			workStatuses.FromMap(m)
			resp.WorkConfig = *workStatuses
		}

		s.deviceInfo.AppendCommonInfo(resp)

		return datamodel.NewModelSendEvent(resp), nil
	}

	rsub, err := action.Register(ctx, action.NewAction(cbRepo), s.newSubConfig(agent.RepoRequestModelName.String()))
	if err != nil {
		return nil, err
	}

	csub, err := action.Register(ctx, action.NewAction(cbCalendar), s.newSubConfig(agent.CalendarRequestModelName.String()))
	if err != nil {
		return nil, err
	}

	psub, err := action.Register(ctx, action.NewAction(cbProject), s.newSubConfig(agent.ProjectRequestModelName.String()))
	if err != nil {
		return nil, err
	}

	wsub, err := action.Register(ctx, action.NewAction(cbWorkconfig), s.newSubConfig(agent.WorkStatusRequestModelName.String()))
	if err != nil {
		panic(err)
	}

	rsub.WaitForReady()
	csub.WaitForReady()
	psub.WaitForReady()
	wsub.WaitForReady()

	return func() {
		rsub.Close()
		csub.Close()
		psub.Close()
		wsub.Close()
	}, nil
}

func (s *runner) getOnboardData(ctx context.Context, config inconfig.IntegrationAgent, messageID string, objectType string) (res cmdexportonboarddata.Result, _ error) {
	s.logger.Info("getting onboarding data for integration", "name", config.Name, "objectType", objectType)

	integrations := []inconfig.IntegrationAgent{config}

	c, err := subcommand.New(subcommand.Opts{
		Logger:            s.logger,
		Tmpdir:            s.fsconf.Temp,
		IntegrationConfig: s.agentConfig,
		AgentConfig:       s.conf,
		Integrations:      integrations,
		DeviceInfo:        s.deviceInfo,
	})

	if err != nil {
		return res, err
	}

	err = c.Run(ctx, "export-onboard-data", messageID, &res, "--object-type", objectType)

	s.logger.Info("getting onboard data completed", "success", res.Success, "err", res.Error)
	if err != nil {
		return res, err
	}

	return res, nil
}
