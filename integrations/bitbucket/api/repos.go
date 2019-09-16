package api

import (
	"net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func ReposOnboardPage(qc QueryContext, teamName string, params url.Values) (page PageInfo, repos []*agent.RepoResponseRepos, err error) {
	qc.Logger.Debug("onboard repos request")

	objectPath := pstrings.JoinURL("teams", teamName, "repositories")
	params.Set("pagelen", "100")

	var rr []struct {
		UUID        string    `json:"uuid"`
		FullName    string    `json:"full_name"`
		Description string    `json:"description"`
		Language    string    `json:"language"`
		CreatedOn   time.Time `json:"created_on"`
	}

	page, err = qc.Request(objectPath, params, true, &rr)
	if err != nil {
		return
	}

	for _, v := range rr {
		repo := &agent.RepoResponseRepos{
			RefID:       v.UUID,
			RefType:     qc.RefType,
			Name:        v.FullName,
			Description: v.Description,
			Language:    v.Language,
			Active:      true,
		}

		repo.LastCommit, err = LastCommit(qc, repo)
		if err != nil {
			return
		}

		date.ConvertToModel(v.CreatedOn, &repo.CreatedDate)

		repos = append(repos, repo)
	}

	return
}
