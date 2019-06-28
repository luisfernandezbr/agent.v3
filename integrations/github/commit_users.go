package main

import (
	"sync"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
)

func (s *Integration) exportCommitAuthors(repoIDs []string, concurrency int) error {
	et, err := s.newExportType("sourcecode.commit_user")
	if err != nil {
		return err
	}
	defer et.Done()

	wg := sync.WaitGroup{}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repoID := range stringsToChan(repoIDs) {
				err := s.exportCommitsForRepo(et, repoID)
				if err != nil {
					panic(err)
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

func (s *Integration) exportCommitsForRepo(et *exportType, repoID string) error {
	branches := make(chan []string)
	go func() {
		defer close(branches)
		err := api.BranchNames(s.qc, repoID, branches)
		if err != nil {
			panic(err)
		}
	}()
	for sl := range branches {
		for _, branch := range sl {
			err := s.exportCommitsForRepoBranch(et, repoID, branch)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Integration) exportCommitsForRepoBranch(et *exportType, repoID string, branchName string) error {
	s.logger.Info("exporting commits for branch", "repoID", repoID, "branch", branchName)

	return api.PaginateCommits(
		et.lastProcessed,
		func(query string) (api.PageInfo, error) {

			pi, res, err := api.CommitsPage(s.qc,
				repoID,
				branchName,
				query,
			)
			if err != nil {
				return pi, err
			}

			s.logger.Info("got commits page", "l", len(res))
			batch := []rpcdef.ExportObj{}

			for _, commit := range res {
				author := CommitUser{}
				author.CustomerID = s.customerID
				author.Name = commit.AuthorName
				author.Email = commit.AuthorEmail
				author.SourceID = commit.AuthorRefID
				batch = append(batch, rpcdef.ExportObj{Data: author.ToMap()})

				committer := CommitUser{}
				committer.CustomerID = s.customerID
				committer.Name = commit.CommitterName
				committer.Email = commit.CommitterEmail
				committer.SourceID = commit.CommitterRefID
				batch = append(batch, rpcdef.ExportObj{Data: committer.ToMap()})
			}

			return pi, et.Send(batch)
		})
}

type CommitUser struct {
	CustomerID string
	Email      string
	Name       string
	SourceID   string
}

func (s CommitUser) ToMap() map[string]interface{} {
	res := map[string]interface{}{}
	res["customer_id"] = s.CustomerID
	res["email"] = s.Email
	res["name"] = s.Name
	res["source_id"] = s.SourceID
	return res
}
