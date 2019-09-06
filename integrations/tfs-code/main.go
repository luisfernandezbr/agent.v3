package main

import (
	"context"
	"fmt"

	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/integrations/tfs-code/api"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/agent.next/rpcdef"

	"github.com/hashicorp/go-hclog"
)

type OtherConfig struct {
	Excluded            []string `json:"excluded_repo_ids"` // excluded repo ids - this comes from the admin ui
	Repos               []string `json:"repo_names"`        // repo_names - this is for debug and develop only
	OverrideGitHostName string   `json:"git_host_name"`

	customerid string
	reftype    string
}

type Integration struct {
	logger hclog.Logger
	agent  rpcdef.Agent
	api    *api.TFSAPI
	creds  *api.Creds
	conf   *OtherConfig
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	return nil
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, err error) {
	if err = s.initConfig(ctx, config); err != nil {
		return
	}
	if err = s.export(); err != nil {
		return
	}
	return
}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {
	if err := s.initConfig(ctx, config); err != nil {
		return res, err
	}
	if _, _, err := s.fetcfReposAndProjectIDs(); err != nil {
		res.Errors = append(res.Errors)
	}
	return res, nil
}

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (rpcdef.OnboardExportResult, error) {
	var res rpcdef.OnboardExportResult
	if err := s.initConfig(ctx, config); err != nil {
		return res, err
	}
	switch objectType {
	case rpcdef.OnboardExportTypeUsers:
		return s.onboardExportUsers(ctx, config)
	case rpcdef.OnboardExportTypeRepos:
		return s.onboardExportRepos(ctx, config)
	default:
		res.Error = rpcdef.ErrOnboardExportNotSupported
	}
	return res, nil
}

func (s *Integration) initConfig(ctx context.Context, config rpcdef.ExportConfig) error {
	var creds api.Creds
	var conf OtherConfig
	if err := structmarshal.MapToStruct(config.Integration, &creds); err != nil {
		return err
	}
	if err := structmarshal.MapToStruct(config.Integration, &conf); err != nil {
		return err
	}
	if creds.Collection == "" {
		creds.Collection = "DefaultCollection"
	}
	if creds.URL == "" {
		return fmt.Errorf("missing url")
	}
	if creds.Username == "" {
		return fmt.Errorf("missing username")
	}
	if creds.Password == "" {
		return fmt.Errorf("missing password")
	}
	if creds.APIKey == "" {
		return fmt.Errorf("missing api_key")
	}

	s.conf = &conf
	s.conf.customerid = config.Pinpoint.CustomerID
	s.conf.reftype = "tfs"
	s.creds = &creds

	s.api = api.NewTFSAPI(ctx, s.logger, s.conf.customerid, s.conf.reftype, &creds)

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
