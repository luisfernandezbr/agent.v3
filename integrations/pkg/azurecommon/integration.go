package azurecommon

import (
	"context"
	"fmt"
	"strings"

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

// IntegrationType the type of integration
type IntegrationType string

func (r IntegrationType) String() string {
	return string(r)
}

var IntegrationTypeCode = IntegrationType("code")

var IntegrationTypeIssues = IntegrationType("issues")

var IntegrationTypeBoth = IntegrationType("")

// Integration the main integration object
type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	api        *azureapi.API
	Creds      *azureapi.Creds `json:"credentials"`
	customerid string
	reftype    RefType

	IntegrationType     IntegrationType `json:"type"`
	ExcludedRepoIDs     []string        `json:"excluded_repo_ids"` // excluded repo ids - this comes from the admin ui
	IncludedRepos       []string        `json:"repo_names"`        // repo_names - this is for debug and develop only
	OverrideGitHostName string          `json:"git_host_name"`
	Concurrency         int             `json:"concurrency"` // add it
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
		a := azureapi.NewAsync(2)
		var errors []string
		a.Send(func() {
			if err = s.exportCode(); err != nil {
				errors = append(errors, err.Error())
			}
		})
		a.Send(func() {
			if err = s.exportWork(); err != nil {
				errors = append(errors, err.Error())
			}
		})
		a.Wait()
		if errors != nil {
			err = fmt.Errorf("%v", strings.Join(errors, ", "))
		}
	}
	return
}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {
	s.logger.Info("========= ValidateConfig")
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

	// itype IntegrationType
	if err := structmarshal.MapToStruct(config.Integration, &s); err != nil {
		return err
	}
	if s.reftype == RefTypeTFS {
		if s.Creds.Collection == nil {
			s.Creds.Collection = pstrings.Pointer("DefaultCollection")
		}
		if s.Creds.Username == "" {
			return fmt.Errorf("missing username")
		}
		if s.Creds.Password == "" {
			return fmt.Errorf("missing password")
		}
	} else { // if s.reftype == RefTypeAzure
		if s.Creds.Organization == nil {
			return fmt.Errorf("missing organization %s", stringify(s))
		}
		if s.Creds.Username == "" {
			s.Creds.Username = *s.Creds.Organization
		}
		if s.Creds.Password == "" {
			s.Creds.Password = s.Creds.APIKey
		}
	}
	if s.Concurrency == 0 {
		s.Concurrency = 10
	}
	if s.IntegrationType != IntegrationTypeBoth && s.IntegrationType != IntegrationTypeCode && s.IntegrationType != IntegrationTypeIssues {
		return fmt.Errorf(`"type" must be "` + IntegrationTypeIssues.String() + `", "` + IntegrationTypeCode.String() + `", or empty for both`)
	}
	if s.Creds.URL == "" {
		return fmt.Errorf("missing url")
	}
	if s.Creds.APIKey == "" {
		return fmt.Errorf("missing api_key")
	}
	s.customerid = config.Pinpoint.CustomerID
	s.logger.Info("Concurrency " + fmt.Sprintf("%d", s.Concurrency))
	s.api = azureapi.NewAPI(ctx, s.logger, s.Concurrency, s.customerid, s.reftype.String(), s.Creds)
	return nil
}

func NewTFSIntegration(logger hclog.Logger, itype IntegrationType) *Integration {
	s := &Integration{}
	s.logger = logger
	s.reftype = RefTypeTFS
	s.IntegrationType = itype
	return s
}

func NewAzureIntegration(logger hclog.Logger) *Integration {
	s := &Integration{}
	s.logger = logger
	s.reftype = RefTypeAzure
	return s
}
