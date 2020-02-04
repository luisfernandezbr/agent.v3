package main

import (
	"context"
	"errors"
	"net/url"

	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/integrations/sonarqube/api"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/agent/rpcdef"

	"github.com/hashicorp/go-hclog"
)

var defaultMetrics = []string{
	"complexity", "code_smells",
	"new_code_smells", "sqale_rating",
	"reliability_rating", "security_rating",
	"coverage", "new_coverage",
	"test_success_density", "new_technical_debt",
}

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	customerID string
	api        *api.SonarqubeAPI
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	return nil
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, _ error) {

	if err := s.initConfig(ctx, config); err != nil {
		return res, err
	}
	if err := s.exportAll(); err != nil {
		return res, err
	}
	return res, nil
}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {
	if err := s.initConfig(ctx, config); err != nil {
		return res, err
	}

	serverVersion, err := s.api.ServerVersion()
	if err != nil {
		return res, err
	}

	res.ServerVersion = serverVersion

	valid, err := s.api.Validate()
	if err != nil {
		res.Errors = append(res.Errors, "Sonarqube validation failed. Error: "+err.Error())
		return res, nil
	}
	// we might have an invalid validation without an error
	if !valid {
		res.Errors = append(res.Errors, "Sonarqube validation failed, probably wrong api token or url")
		return res, nil
	}
	return res, nil
}

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	res.Error = rpcdef.ErrOnboardExportNotSupported
	return
}

func (s *Integration) initConfig(ctx context.Context, config rpcdef.ExportConfig) error {

	var defConfig struct {
		URL     string   `json:"url"`
		APIKey  string   `json:"api_key"`
		Metrics []string `json:"metrics"`
	}

	err := structmarshal.MapToStruct(config.Integration.Config, &defConfig)
	if err != nil {
		return err
	}
	if defConfig.URL == "" {
		return errors.New("url missing")
	}
	purl := defConfig.URL

	if _, err := url.ParseRequestURI(purl); err != nil {
		return errors.New("invalid url")
	}

	if defConfig.APIKey == "" {
		return errors.New("api_key missing")
	}
	apikey := defConfig.APIKey

	metrics := defConfig.Metrics
	if len(metrics) == 0 {
		metrics = defaultMetrics
	}
	s.api = api.NewSonarqubeAPI(ctx, s.logger, purl, apikey, metrics)
	s.customerID = config.Pinpoint.CustomerID
	return nil
}

func NewIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	return s
}

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return NewIntegration(logger)
	})
}
