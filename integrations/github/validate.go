package main

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/integrations/github/api"
	purl "github.com/pinpt/agent.next/integrations/pkg/url"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) checkEnterpriseVersion() error {
	version, err := api.EnterpriseVersion(s.qc, s.config.APIURL)
	if err != nil {
		return fmt.Errorf("could not get the version of your github install, err: %v", err)
	}
	if version <= "2.15" {
		return fmt.Errorf("the version of your github install is too old, version: %v", version)
	}
	s.logger.Info("github enterprise version is", "v", version)
	return nil
}

func (s *Integration) ValidateConfig(ctx context.Context,
	exportConfig rpcdef.ExportConfig) (res rpcdef.ValidationResult, _ error) {

	rerr := func(err error) {
		res.Errors = append(res.Errors, err.Error())
	}

	err := s.initWithConfig(exportConfig)
	if err != nil {
		rerr(err)
		return
	}

	orgs, err := s.getOrgs()
	if err != nil {
		rerr(err)
		return
	}

	for _, org := range orgs {
		repoName, err := api.GetSingleRepo(s.qc, org.Login)
		if err != nil {
			rerr(err)
			return
		}
		repoURL, err := purl.GetRepoURL(s.config.RepoURLPrefix, url.UserPassword(s.config.Token, ""), repoName, nil)
		if err != nil {
			rerr(err)
			return
		}

		res.ReposUrls = append(res.ReposUrls, repoURL)
	}

	return
}
