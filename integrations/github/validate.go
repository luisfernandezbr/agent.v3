package main

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/agent/rpcdef"
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

var requiredScopes = []string{"read:org", "repo", "user", "read:user", "user:email"}

func (s *Integration) checkTokenScopes() error {
	// https://developer.github.com/apps/building-oauth-apps/understanding-scopes-for-oauth-apps/
	scopes, err := api.TokenScopes(s.qc)
	if err != nil {
		return err
	}
	m := map[string]bool{}
	for _, sc := range scopes {
		m[sc] = true
	}
	scopeErr := func(sc string) error {
		return fmt.Errorf("No required scope %v. Scopes wanted: %v got: %v", sc, requiredScopes, scopes)
	}
	if !m["read:org"] && !m["admin:org"] {
		return scopeErr("read:org")
	}
	if !m["repo"] {
		return scopeErr("repo")
	}
	if !m["user"] {
		if !m["read:user"] {
			return scopeErr("read:user")
		}
		if !m["user:email"] {
			return scopeErr("user:email")
		}
	}
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

	if s.config.Enterprise {
		version, err := api.EnterpriseVersion(s.qc, s.config.APIURL)
		if err != nil {
			rerr(err)
			return
		}
		res.ServerVersion = version
	} else {
		res.ServerVersion = "cloud"
	}

	err = s.checkTokenScopes()
	if err != nil {
		rerr(fmt.Errorf("Token scope err: %v", err))
		return
	}

	orgs, err := s.getOrgs()
	if err != nil {
		rerr(err)
		return
	}

	if len(orgs) == 0 {
		// if no orgs available test user repo
		orgs = []api.Org{{}}
		return
	}

LOOP:
	for _, org := range orgs {
		_, repos, err := api.ReposPageInternal(s.qc, org, "first: 1")
		if err != nil {
			rerr(err)
			return
		}
		if len(repos) > 0 {
			repoURL, err := getRepoURL(s.config.RepoURLPrefix, url.UserPassword(s.config.Token, ""), repos[0].NameWithOwner)
			if err != nil {
				rerr(err)
				return
			}

			res.RepoURL = repoURL
			break LOOP // only return 1 repo url
		}
	}

	return
}
