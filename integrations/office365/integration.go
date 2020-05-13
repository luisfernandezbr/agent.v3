package main

import (
	"context"
	"errors"

	"github.com/pinpt/integration-sdk/agent"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/office365/api"
	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/pkg/oauthtoken"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/agent/rpcdef"
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
	s.refType = "office365"
	return nil
}

// Export exports all the calendars in the Inclusions list and its events
func (s *Integration) Export(ctx context.Context, conf rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {
	return s.export(ctx, conf)
}

// ValidateConfig calls a simple api to make sure we have the correct credentials
func (s *Integration) ValidateConfig(ctx context.Context, conf rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {
	api, err := api.New(s.logger, conf.Pinpoint.CustomerID, s.refType, func() (string, error) {
		oauth, err := oauthtoken.New(s.logger, s.agent)
		return oauth.Get(), err
	})
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}
	return res, api.Validate()
}

// OnboardExport returns the data used in onboard
func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, conf rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	if err := s.initAPI(conf); err != nil {
		res.Error = err
		return res, err
	}
	api, err := api.New(s.logger, conf.Pinpoint.CustomerID, s.refType, func() (string, error) {
		oauth, err := oauthtoken.New(s.logger, s.agent)
		return oauth.Get(), err
	})
	if err != nil {
		res.Error = err
		return res, err
	}
	cals, err := api.GetSharedCalendars()
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

// Webhook not supported
func (s *Integration) Webhook(ctx context.Context, headers map[string]string, body string, config rpcdef.ExportConfig) (res rpcdef.WebhookResult, rerr error) {
	rerr = errors.New("webhook not supported")
	return
}

func (s *Integration) initAPI(conf rpcdef.ExportConfig) error {
	if err := structmarshal.MapToStruct(conf.Integration.Config, &s.config); err != nil {
		s.logger.Error("error creating the config object", "err", err)
		return err
	}
	return nil
}
