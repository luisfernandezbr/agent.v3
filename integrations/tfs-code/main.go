package main

import (
	"context"
	"fmt"

	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/integrations/tfs-code/api"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-common/hash"

	"github.com/hashicorp/go-hclog"
)

type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	api        *api.TFSAPI
	conf       *api.Creds
	customerid string
	reftype    string
}

func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	return nil
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, err error) {
	s.initConfig(ctx, config)
	err = s.export()
	return
}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {
	return res, nil
}

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {
	res.Error = rpcdef.ErrOnboardExportNotSupported
	return
}

func (s *Integration) initConfig(ctx context.Context, config rpcdef.ExportConfig) error {
	var conf api.Creds
	if err := structmarshal.MapToStruct(config.Integration, &conf); err != nil {
		return err
	}
	if conf.Collection == "" {
		conf.Collection = "DefaultCollection"
	}
	if conf.URL == "" {
		return fmt.Errorf("missing url")
	}
	if conf.Username == "" {
		return fmt.Errorf("missing username")
	}
	if conf.Password == "" {
		return fmt.Errorf("missing password")
	}
	if conf.APIKey == "" {
		return fmt.Errorf("missing api_key")
	}
	s.customerid = config.Pinpoint.CustomerID
	s.reftype = "tfs"
	s.conf = &conf
	s.api = api.NewTFSAPI(ctx, s.logger, s.customerid, s.reftype, &conf)
	s.api.RepoID = func(refID string) string {
		return hash.Values("Repo", s.customerid, s.reftype, refID)
	}
	s.api.UserID = func(refID string) string {
		return hash.Values("User", s.customerid, s.reftype, refID)
	}
	s.api.PullRequestID = func(refID string) string {
		return hash.Values("PullRequest", s.customerid, s.reftype, refID)
	}
	s.api.BranchID = func(repoRefID string, branchName string) string {
		repoID := s.api.RepoID(repoRefID)
		return hash.Values(s.reftype, repoID, s.customerid, branchName)
	}

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
