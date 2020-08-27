package api

import (
	"net/url"
	"time"

	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/go-common/v10/datetime"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// ResposUserHasAccessToPage it will fetch repos the user has access to
func ResposUserHasAccessToPage(
	qc QueryContext,
	params url.Values,
	nextPage NextPage) (np NextPage, repos []*agent.RepoResponseRepos, err error) {

	qc.Logger.Debug("onboard repos request", "params", params, "next_page", nextPage)

	objectPath := pstrings.JoinURL("repositories")
	params.Set("pagelen", "100")
	params.Set("role", "member")

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

func ReposAll(qc interface{}, res chan []commonrepo.Repo) error {

	params := url.Values{}
	params.Set("pagelen", "100")

	return Paginate(func(nextPage NextPage) (NextPage, error) {
		pi, repos, err := ReposPage(qc.(QueryContext), params, nextPage)
		if err != nil {
			return pi, err
		}
		res <- repos
		return pi, nil
	})
}

func ReposPage(
	qc QueryContext,
	params url.Values,
	nextPage NextPage) (np NextPage, repos []commonrepo.Repo, err error) {

	params.Set("role", "member")

	qc.Logger.Debug("repos", "params", params)

	objectPath := pstrings.JoinURL("repositories")

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
	qc QueryContext,
	params url.Values,
	stopOnUpdatedAt time.Time,
	nextPage NextPage) (np NextPage, repos []*sourcecode.Repo, err error) {

	qc.Logger.Debug("repos sourcecode", "params", params, "next", nextPage)

	objectPath := pstrings.JoinURL("repositories")
	params.Set("pagelen", "100")
	params.Set("role", "member")

	type repo struct {
		CreatedOn   time.Time `json:"created_on"`
		UpdatedOn   time.Time `json:"updated_on"`
		UUID        string    `json:"uuid"`
		FullName    string    `json:"full_name"`
		Description string    `json:"description"`
		Links       struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	}

	var rr []repo

	np, err = qc.Request(objectPath, params, true, &rr, nextPage)
	if err != nil {
		return
	}

	for _, repo := range rr {
		if stopOnUpdatedAt.After(repo.UpdatedOn) {
			return
		}
		repo := &sourcecode.Repo{
			RefID:       repo.UUID,
			RefType:     qc.RefType,
			CustomerID:  qc.CustomerID,
			Name:        repo.FullName,
			URL:         repo.Links.HTML.Href,
			Description: repo.Description,
			UpdatedAt:   datetime.TimeToEpoch(repo.UpdatedOn),
			Active:      true,
		}

		repos = append(repos, repo)
	}

	return
}
