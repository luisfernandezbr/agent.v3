package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/pinpt/integration-sdk/sourcecode"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/azure/api"
	"github.com/pinpt/agent/integrations/pkg/ibase"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/agent/rpcdef"
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

// Integration the main integration object
type Integration struct {
	logger     hclog.Logger
	agent      rpcdef.Agent
	api        *api.API
	Creds      *api.Creds `json:"credentials"`
	customerid string
	orgSession *objsender.Session

	// RefType switches between azure and tfs
	RefType         RefType         `json:"reftype"`
	IntegrationType IntegrationType `json:"type"`
	// ExcludedRepoIDs this comes from the admin UI
	ExcludedRepoIDs []string `json:"excluded_repos"`
	IncludedRepoIDs []string `json:"included_repos"`
	// ExcludedProjectIDs this comes from the admin UI
	ExcludedProjectIDs []string `json:"excluded_projects"`
	IncludedProjectIDs []string `json:"included_projects"`
	// IncludedRepos names of repos to process. Used for debugging and development.
	Repos               []string `json:"repos"`
	Projects            []string `json:"projects"`
	OverrideGitHostName string   `json:"git_host_name"`
	Concurrency         int      `json:"concurrency"`
}

// Init the init function
func (s *Integration) Init(agent rpcdef.Agent) error {
	s.agent = agent
	return nil
}

func (s *Integration) Export(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ExportResult, rerr error) {

	err := s.initConfig(ctx, config)
	if err != nil {
		rerr = err
		return
	}
	var orgtype string
	if s.RefType == RefTypeTFS {
		orgtype = "collection"
	} else {
		orgtype = "organization"
	}
	if s.orgSession, err = objsender.RootTracking(s.agent, orgtype); err != nil {
		rerr = err
		return
	}

	if s.IntegrationType == IntegrationTypeCode {
		repoResults, err := s.exportCode()
		if err != nil {
			rerr = err
			return
		}
		res.Projects = repoResults
		return
	} else if s.IntegrationType == IntegrationTypeIssues {
		projectResults, err := s.exportWork()
		if err != nil {
			rerr = err
			return
		}
		res.Projects = projectResults
		return
	}

	panic("IntegrationType is required")

}

func (s *Integration) ValidateConfig(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.ValidationResult, err error) {

	if err = s.initConfig(ctx, config); err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}
	var repos []*sourcecode.Repo
	// do a quick api call to see if the credentials, url, etc.. are correct
	if _, repos, err = s.api.FetchAllRepos(s.Repos, s.ExcludedRepoIDs, s.IncludedRepoIDs); err != nil {
		// don't return, get as many errors are possible
		res.Errors = append(res.Errors, err.Error())
		return res, err
	}

	if s.IntegrationType == IntegrationTypeCode {
		// only check git clone if this is a SOURCECODE type
		if len(repos) > 0 {
			repoURL, err := s.appendCredentials(repos[0].URL)
			if err != nil {
				res.Errors = append(res.Errors, err.Error())
				return res, err
			}
			res.RepoURL = repoURL
		}
	}
	return res, err
}

func (s *Integration) OnboardExport(ctx context.Context, objectType rpcdef.OnboardExportType, config rpcdef.ExportConfig) (rpcdef.OnboardExportResult, error) {

	var res rpcdef.OnboardExportResult
	if err := s.initConfig(ctx, config); err != nil {
		return res, err
	}
	switch objectType {
	case rpcdef.OnboardExportTypeRepos:
		return s.onboardExportRepos()
	case rpcdef.OnboardExportTypeProjects:
		return s.onboardExportProjects()
	case rpcdef.OnboardExportTypeWorkConfig:
		return s.onboardWorkConfig()
	default:
		s.logger.Error("objectType not supported", "objectType", objectType)
		res.Error = rpcdef.ErrOnboardExportNotSupported
	}
	return res, nil
}

func (s *Integration) initConfig(ctx context.Context, config rpcdef.ExportConfig) (err error) {
	err = structmarshal.StructToStruct(config.Integration.Config, &s.Creds)
	if err != nil {
		return err
	}
	if s.Creds.Organization != "" {
		s.RefType = RefTypeAzure

		if s.Creds.Username == "" {
			s.Creds.Username = s.Creds.Organization
		}

		if s.Creds.Password == "" {
			s.Creds.Password = s.Creds.APIKey
		}

	} else if s.Creds.CollectionName != "" {
		s.RefType = RefTypeTFS

		if s.Creds.Username == "" {
			return errors.New("missing username")
		}

		if s.Creds.Password == "" {
			return errors.New("missing password")
		}

	} else {
		return fmt.Errorf("missing organization or collection %s", stringify(config.Integration.Config))
	}

	if s.Creds.URL == "" {
		return fmt.Errorf("missing url %s", stringify(config.Integration.Config))
	}

	if s.Creds.APIKey == "" {
		return fmt.Errorf("missing api key %s", stringify(config.Integration.Config))
	}
	s.IntegrationType = IntegrationType(config.Integration.Type.String())

	if s.IntegrationType != IntegrationTypeCode && s.IntegrationType != IntegrationTypeIssues {
		return errors.New(`"type" must be "` + IntegrationTypeIssues.String() + `", "` + IntegrationTypeCode.String() + `"`)
	}
	s.Concurrency = 10
	s.customerid = config.Pinpoint.CustomerID
	s.api = api.NewAPI(ctx, s.logger, s.Concurrency, s.customerid, s.RefType.String(), s.Creds, s.RefType == RefTypeTFS)
	return nil
}

func main() {
	ibase.MainFunc(func(logger hclog.Logger) rpcdef.Integration {
		return &Integration{
			logger: logger,
		}
	})
}
