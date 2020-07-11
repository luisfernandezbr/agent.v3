package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/bitbucket/api"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func FilteredRepos(qc interface{}, teamName string, inclusions []string) (repos []commonrepo.Repo, err error) {

	params := url.Values{}

	appendQueryParameter(params, inclusions)

	_, repos, err = api.ReposPage(qc.(api.QueryContext), teamName, params, "")
	return
}

func (s *Integration) exportRepos(ctx context.Context, logger hclog.Logger, sender *objsender.Session, groupName string, srepos []*sourcecode.Repo) error {

	if len(srepos) > 0 {
		for _, repo := range srepos {
			if err := sender.Send(repo); err != nil {
				return err
			}
		}
		return nil
	}

	stopOnUpdatedAt := sender.LastProcessedTime()

	params := url.Values{}
	params.Set("pagelen", "100")
	return api.Paginate(func(nextPage api.NextPage) (api.NextPage, error) {
		np, repos, err := api.ReposSourcecodePage(s.qc, groupName, params, stopOnUpdatedAt, nextPage)
		if err != nil {
			return np, err
		}

		for _, repo := range repos {
			if err := sender.Send(repo); err != nil {
				return np, err
			}
		}
		return np, nil
	})
}

func appendQueryParameter(params url.Values, onlyInclude []string) {

	repoRefIDFilters := make([]string, 0)
	for _, repo := range onlyInclude {
		repoRefIDFilters = append(repoRefIDFilters, fmt.Sprintf("uuid = \"%s\"", repo))
	}

	params.Set("q", fmt.Sprintf("(%s)", strings.Join(repoRefIDFilters, " OR ")))
}
