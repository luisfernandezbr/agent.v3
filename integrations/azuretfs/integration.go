package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pinpt/go-common/datamodel"
	pstrings "github.com/pinpt/go-common/strings"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/azuretfs/api"
	"github.com/pinpt/agent.next/integrations/pkg/ibase"
	"github.com/pinpt/agent.next/pkg/structmarshal"
	"github.com/pinpt/agent.next/rpcdef"
)

// RefType the type of integration
type RefType string

func (r RefType) String() string {
	return string(r)
}

// RefTypeTFS the tfs integration type
var RefTypeTFS = RefType("tfs")

// RefTypeAzure the azure integration type
var RefTypeAzure = RefType("azure")

// IntegrationType the type of integration
type IntegrationType string

func (r IntegrationType) String() string {
	return string(r)
}

var IntegrationTypeCode = IntegrationType("SOURCECODE")

var IntegrationTypeIssues = IntegrationType("WORK")

var IntegrationTypeBoth = IntegrationType("")

// Integration the main integration object
type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	api        *api.API
	Creds      *api.Creds `json:"credentials"`
	customerid string

	// RefType switches between azure and tfs
	RefType         RefType         `json:"reftype"`
	IntegrationType IntegrationType `json:"type"`
	// ExcludedRepoIDs this comes from the admin UI
	ExcludedRepoIDs []string `json:"excluded_repo_ids"`
	// IncludedRepos names of repos to process. Used for debugging and development.
	IncludedRepos       []string `json:"included_repos"`
	OverrideGitHostName string   `json:"git_host_name"`
	Concurrency         int      `json:"concurrency"`
}

// Init the init function
func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	return nil
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, err error) {
	if err = s.initConfig(ctx, config); err != nil {
		return
	}
	if s.IntegrationType == IntegrationTypeCode {
		err = s.exportCode()
	} else if s.IntegrationType == IntegrationTypeIssues {
		err = s.exportWork()
	} else {
		async := api.NewAsync(2)
		var errors []string
		async.Do(func() {
			if err = s.exportCode(); err != nil {
				errors = append(errors, err.Error())
			}
		})
		async.Do(func() {
			if err = s.exportWork(); err != nil {
				errors = append(errors, err.Error())
			}
		})
		async.Wait()
		if errors != nil {
			err = fmt.Errorf("%v", strings.Join(errors, ", "))
		}
	}
	return
}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, err error) {
	if err = s.initConfig(ctx, config); err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}
	repochan, done := api.AsyncProcess("validate", s.logger, func(m datamodel.Model) {
		// empty, nothing to do here since we're just validating
	})
	if _, err = s.api.FetchAllRepos(s.IncludedRepos, s.ExcludedRepoIDs, repochan); err != nil {
		// don't return, get as many errors are possible
		res.Errors = append(res.Errors, err.Error())
	}
	close(repochan)
	<-done
	return res, err
}

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (rpcdef.OnboardExportResult, error) {
	var res rpcdef.OnboardExportResult
	if err := s.initConfig(ctx, config); err != nil {
		return res, err
	}
	switch objectType {
	case rpcdef.OnboardExportTypeRepos:
		return s.onboardExportRepos(ctx, config)
	case rpcdef.OnboardExportTypeProjects:
		return s.onboardExportProjects(ctx, config)
	default:
		s.logger.Error("objectType not supported", "objectType", objectType)
		res.Error = rpcdef.ErrOnboardExportNotSupported
	}
	return res, nil
}

func (s *Integration) initConfig(ctx context.Context, config rpcdef.ExportConfig) error {
	// type IntegrationType
	if err := structmarshal.MapToStruct(config.Integration, &s); err != nil {
		return err
	}
	var istfs bool
	if s.RefType == RefTypeTFS {
		istfs = true
		if s.Creds.Collection == nil {
			s.Creds.Collection = pstrings.Pointer("DefaultCollection")
		}
		if s.Creds.Username == "" {
			return errors.New("missing username")
		}
		if s.Creds.Password == "" {
			return errors.New("missing password")
		}
	} else if s.RefType == RefTypeAzure {
		if s.Creds.Organization == nil {
			return fmt.Errorf("missing organization %s", stringify(s))
		}
		if s.Creds.Username == "" {
			s.Creds.Username = *s.Creds.Organization
		}
		if s.Creds.Password == "" {
			s.Creds.Password = s.Creds.APIKey
		}
	} else {
		return errors.New(`"retype" must be "` + RefTypeTFS.String() + `" or "` + RefTypeAzure.String())
	}
	if s.Concurrency == 0 {
		s.Concurrency = 10
	}
	if s.IntegrationType != IntegrationTypeBoth && s.IntegrationType != IntegrationTypeCode && s.IntegrationType != IntegrationTypeIssues {
		return errors.New(`"type" must be "` + IntegrationTypeIssues.String() + `", "` + IntegrationTypeCode.String() + `", or empty for both`)
	}
	if s.Creds.URL == "" {
		return errors.New("missing url")
	}
	if s.Creds.APIKey == "" {
		return errors.New("missing api_key")
	}
	s.customerid = config.Pinpoint.CustomerID
	s.api = api.NewAPI(ctx, s.logger, s.Concurrency, s.customerid, s.RefType.String(), s.Creds, istfs)
	return nil
}

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return &Integration{
			logger: logger,
		}
	})
}
