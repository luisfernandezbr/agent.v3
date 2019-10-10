package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pinpt/integration-sdk/work"

	"github.com/pinpt/agent.next/integrations/azuretfs/api"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) onboardExportRepos(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, err error) {

	var repos []*sourcecode.Repo
	_, repos, err = s.api.FetchAllRepos([]string{}, []string{})
	if err != nil {
		s.logger.Error("error fetching repos for onboard export repos")
		return
	}
	for _, repo := range repos {
		rawcommit, err := s.api.FetchLastCommit(repo.RefID)
		if rawcommit == nil {
			s.logger.Error("last commit is nil, skipping", "repo ref_id", repo.RefID)
			continue
		}
		if err != nil {
			s.logger.Error("error fetching last commit for onboard, skipping", "repo ref_id", repo.RefID, "err", err)
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
				CommitID:  ids.CodeCommit(s.customerid, s.RefType.String(), repo.ID, rawcommit.CommitID),
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

func (s *Integration) onboardExportProjects(ctx context.Context, config rpcdef.ExportConfig) (res rpcdef.OnboardExportResult, err error) {
	projectchan, done := api.AsyncProcess("export projects", s.logger, func(model datamodel.Model) {
		proj := model.(*work.Project)
		itemids, err := s.api.FetchItemIDs(proj.RefID, time.Time{})
		if err != nil {
			s.logger.Error("error getting issue count", "err", err)
			return
		}
		itemchan := make(chan datamodel.Model, 1)
		raw, err := s.api.FetchWorkItemsByIDs(proj.RefID, append([]string{}, itemids[len(itemids)-1]), itemchan)
		if err != nil {
			s.logger.Error("error getting last issue", "err", err)
			return
		}
		lastitem := (<-itemchan).(*work.Issue)
		resp := &agent.ProjectResponseProjects{
			Active:     proj.Active,
			Identifier: proj.Identifier,
			LastIssue: agent.ProjectResponseProjectsLastIssue{
				IssueID:     ids.WorkIssue(s.customerid, proj.RefType, fmt.Sprintf("%d", raw[0].ID)),
				Identifier:  lastitem.Identifier,
				CreatedDate: agent.ProjectResponseProjectsLastIssueCreatedDate(lastitem.CreatedDate),
				LastUser: agent.ProjectResponseProjectsLastIssueLastUser{
					UserID:    raw[0].Fields.CreatedBy.ID,
					Name:      raw[0].Fields.CreatedBy.DisplayName,
					AvatarURL: raw[0].Fields.CreatedBy.ImageURL,
				},
			},
			Name:        proj.Name,
			RefID:       proj.RefID,
			RefType:     proj.RefType,
			TotalIssues: int64(len(itemids)),
			URL:         proj.URL,
		}
		res.Records = append(res.Records, resp.ToMap())
	})
	_, err = s.api.FetchProjects(projectchan)
	close(projectchan)
	<-done
	return res, err
}
