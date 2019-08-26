package main

import (
	"context"
	"os"

	"github.com/pinpt/agent.next/integrations/sonarqube/api"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/agent.next/rpcdef"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

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
	s.initConfig(ctx, config)
	valid, err := s.api.Validate()
	if !valid {
		res.Errors = append(res.Errors, "example validation error")
		return res, nil
	}
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, nil

	}
	return res, nil
}

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	s.initConfig(ctx, config)
	// TODO:
	return res, nil
}

func (s *Integration) initConfig(ctx context.Context, config rpcdef.ExportConfig) error {
	var m map[string]interface{}
	var err error
	if m, err = structmarshal.StructToMap(config.Integration); err != nil {
		return err
	}
	var conf struct {
		URL       string   `json:"url"`
		AuthToken string   `json:"api_token"`
		Metrics   []string `json:"metrics"`
	}
	if err = structmarshal.MapToStruct(m, &conf); err != nil {
		return err
	}
	s.api = api.NewSonarqubeAPI(ctx, conf.URL, conf.AuthToken, conf.Metrics)
	s.customerID = config.Pinpoint.CustomerID
	return nil
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	integration := &Integration{
		logger: logger,
	}

	var pluginMap = map[string]plugin.Plugin{
		"integration": &rpcdef.IntegrationPlugin{Impl: integration},
	}

	logger.Info("loading Sonarqube integration")

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
