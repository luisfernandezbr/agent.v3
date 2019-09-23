package azurecommon

import (
	"context"

	"github.com/pinpt/agent.next/integrations/pkg/azureapi"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) onboardExportUsers(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, _ error) {

	repochan := make(chan datamodel.Model)
	projids, err := s.api.FetchAllRepos([]string{}, []string{}, repochan)
	close(repochan)
	if err != nil {
		s.logger.Error("error fetching repos for onboard export users")
		return
	}
	usermap := make(map[string]*sourcecode.User)
	for _, projid := range projids {
		teamids, err := s.api.FetchTeamIDs(projid)
		if err != nil {
			s.logger.Error("error fetching team ids for users for onboard export")
			return
		}
		err = s.api.FetchSourcecodeUsers(projid, teamids, usermap)
		if err != nil {
			s.logger.Error("error fetching users for onboard export")
			return
		}
	}
	for _, user := range usermap {
		u := agent.UserResponseUsers{
			RefType:    user.RefType,
			RefID:      user.RefID,
			CustomerID: user.CustomerID,
			AvatarURL:  user.AvatarURL,
			Name:       user.Name,
			Username:   *user.Username,
			Active:     true,
		}
		if user.Email != nil {
			u.Emails = []string{*user.Email}
		}
		res.Records = append(res.Records, u.ToMap())
	}
	return res, nil
}

func (s *Integration) onboardExportRepos(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, err error) {

	var repos []*sourcecode.Repo
	reposchan, done := azureapi.AsyncProcess("export repos", s.logger, func(model datamodel.Model) {
		repos = append(repos, model.(*sourcecode.Repo))
	})
	_, err = s.api.FetchAllRepos([]string{}, []string{}, reposchan)
	close(reposchan)
	<-done
	if err != nil {
		s.logger.Error("error fetching repos for onboard export repos")
		return
	}
	for _, repo := range repos {
		rawcommit, err := s.api.FetchLastCommit(repo.RefID)
		if err != nil {
			s.logger.Error("error fetching last commit for onboard, skipping", "repo ref_id", repo.RefID)
			continue
		}
		r := &agent.RepoResponseRepos{
			Active:      repo.Active,
			Description: repo.Description,
			Language:    repo.Language,
			LastCommit: agent.RepoResponseReposLastCommit{
				Author: agent.RepoResponseReposLastCommitAuthor{
					Name:  rawcommit.Author.Name,
					Email: rawcommit.Author.Email,
				},
				CommitSha: rawcommit.CommitID,
				CommitID:  ids.CodeCommit(s.customerid, s.reftype.String(), repo.ID, rawcommit.CommitID),
				URL:       rawcommit.URL,
				Message:   rawcommit.Comment,
			},
			Name:    repo.Name,
			RefID:   repo.RefID,
			RefType: repo.RefType,
		}
		date.ConvertToModel(rawcommit.Author.Date, &r.LastCommit.CreatedDate)
		res.Records = append(res.Records, r.ToMap())
	}
	return
}
