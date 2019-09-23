package azurecommon

import (
	"context"
	"fmt"

	"github.com/pinpt/go-common/datamodel"
	pstrings "github.com/pinpt/go-common/strings"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/pkg/azureapi"
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

// RefType the type of integration
type IntegrationType string

func (r IntegrationType) String() string {
	return string(r)
}

// RefTypeTFS the tfs integration type
var IntegrationTypeCode = IntegrationType("code")

// RefTypeAzure the azure integration type
var IntegrationTypeIssues = IntegrationType("issues")

// Integration the main integration object
type Integration struct {
	logger          hclog.Logger
	agent           rpcdef.Agent
	api             *azureapi.API
	creds           *azureapi.Creds
	customerid      string
	reftype         RefType
	integrationtype IntegrationType

	ExcludedRepoIDs     []string `json:"excluded_repo_ids"` // excluded repo ids - this comes from the admin ui
	IncludedRepos       []string `json:"repo_names"`        // repo_names - this is for debug and develop only
	OverrideGitHostName string   `json:"git_host_name"`
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
	if s.integrationtype == IntegrationTypeCode {
		err = s.exportCode()
	} else {
		err = s.exportWork()
	}
	if err != nil {
		panic(err)
	}
	return
}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {
	if err := s.initConfig(ctx, config); err != nil {
		return res, err
	}
	repochan, done := azureapi.AsyncProcess("validate", s.logger, func(m datamodel.Model) {
		// empty, nothing to do here
	})
	if _, err := s.api.FetchAllRepos(s.IncludedRepos, s.ExcludedRepoIDs, repochan); err != nil {
		res.Errors = append(res.Errors)
	}
	close(repochan)
	<-done
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
	if err := structmarshal.MapToStruct(config.Integration, &s.creds); err != nil {
		return err
	}
	if err := structmarshal.MapToStruct(config.Integration, s); err != nil {
		return err
	}
	if s.reftype == RefTypeTFS {
		if s.creds.Collection == nil {
			s.creds.Collection = pstrings.Pointer("DefaultCollection")
		}
		if s.creds.Username == "" {
			return fmt.Errorf("missing username")
		}
		if s.creds.Password == "" {
			return fmt.Errorf("missing password")
		}
	} else { // if s.reftype == RefTypeAzure
		if s.creds.Organization == nil {
			return fmt.Errorf("missing organization")
		}
		if s.creds.Username == "" {
			s.creds.Username = *s.creds.Organization
		}
		if s.creds.Password == "" {
			s.creds.Password = s.creds.APIKey
		}
	}
	if s.creds.URL == "" {
		return fmt.Errorf("missing url")
	}
	if s.creds.APIKey == "" {
		return fmt.Errorf("missing api_key")
	}
	s.customerid = config.Pinpoint.CustomerID
	s.api = azureapi.NewAPI(ctx, s.logger, s.customerid, s.reftype.String(), s.creds)
	return nil
}

func NewTFSIntegration(logger hclog.Logger, itype IntegrationType) *Integration {
	s := &Integration{}
	s.logger = logger
	s.reftype = RefTypeTFS
	s.integrationtype = itype
	return s
}

func NewAzureIntegration(logger hclog.Logger, itype IntegrationType) *Integration {
	s := &Integration{}
	s.logger = logger
	s.reftype = RefTypeAzure
	s.integrationtype = itype
	return s
}
