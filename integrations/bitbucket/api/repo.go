package api

import (
	"net/url"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/go-common/datetime"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func ReposOnboardPage(
	qc QueryContext,
	teamName string,
	params url.Values,
	nextPage NextPage) (np NextPage, repos []*agent.RepoResponseRepos, err error) {
	qc.Logger.Debug("onboard repos request", "teamName", teamName, "params", params.Encode())

	objectPath := pstrings.JoinURL("repositories", teamName)
	params.Set("pagelen", "100")

	var rr []struct {
		UUID        string    `json:"uuid"`
		FullName    string    `json:"full_name"`
		Description string    `json:"description"`
		Language    string    `json:"language"`
		CreatedOn   time.Time `json:"created_on"`
	}

	np, err = qc.Request(objectPath, params, true, &rr, nextPage)
	if err != nil {
		return
	}

	for _, v := range rr {
		repo := &agent.RepoResponseRepos{
			Active:      true,
			RefID:       v.UUID,
			RefType:     qc.RefType,
			Name:        v.FullName,
			Description: v.Description,
			Language:    v.Language,
		}

		date.ConvertToModel(v.CreatedOn, &repo.CreatedDate)

		repos = append(repos, repo)
	}

	return
}

func ReposAll(qc interface{}, teamName string, res chan []commonrepo.Repo) error {

	params := url.Values{}
	params.Set("pagelen", "100")

	return Paginate(qc.(QueryContext).Logger, func(log hclog.Logger, nextPage NextPage) (NextPage, error) {
		pi, repos, err := ReposPage(qc.(QueryContext), teamName, params, nextPage)
		if err != nil {
			return pi, err
		}
		res <- repos
		return pi, nil
	})
}

func ReposPage(qc QueryContext, teamName string, params url.Values, nextPage NextPage) (np NextPage, repos []commonrepo.Repo, err error) {
	qc.Logger.Debug("repos request repos page", "team", teamName, "params", params.Encode())

	objectPath := pstrings.JoinURL("repositories", teamName)

	var rr []struct {
		UUID       string `json:"uuid"`
		FullName   string `json:"full_name"`
		MainBranch struct {
			Name string `json:"name"`
		} `json:"mainbranch"`
	}

	np, err = qc.Request(objectPath, params, true, &rr, nextPage)
	if err != nil {
		return
	}

	for _, repo := range rr {
		repo := commonrepo.Repo{
			RefID:         repo.UUID,
			NameWithOwner: repo.FullName,
			DefaultBranch: repo.MainBranch.Name,
		}

		repos = append(repos, repo)
	}

	return
}

func ReposSourcecodePage(
	qc QueryContext, teamName string,
	params url.Values,
	stopOnUpdatedAt time.Time,
	nextPage NextPage) (np NextPage, repos []*sourcecode.Repo, err error) {

	qc.Logger.Debug("repos request repos sourcecode page", "teamName", teamName)

	objectPath := pstrings.JoinURL("repositories", teamName)

	type repo struct {
		CreatedAt   time.Time `json:"created_on"`
		UpdatedAt   time.Time `json:"updated_on"`
		UUID        string    `json:"uuid"`
		FullName    string    `json:"full_name"`
		Description string    `json:"description"`
		Links       struct {
			Html struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	}

	var rr []repo

	np, err = qc.Request(objectPath, params, true, &rr, nextPage)
	if err != nil {
		return
	}

	var processRepos []repo

	for _, r := range rr {
		if r.UpdatedAt.After(stopOnUpdatedAt) {
			processRepos = append(processRepos, r)
		}
	}

	for _, repo := range processRepos {
		repo := &sourcecode.Repo{
			RefID:       repo.UUID,
			RefType:     qc.RefType,
			CustomerID:  qc.CustomerID,
			Name:        repo.FullName,
			URL:         repo.Links.Html.Href,
			Description: repo.Description,
			UpdatedAt:   datetime.TimeToEpoch(repo.UpdatedAt),
			Active:      true,
		}

		repos = append(repos, repo)
	}

	return
}
