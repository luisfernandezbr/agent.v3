package main

import (
	"sync"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func (s *Integration) exportCommits(repoIDs []string, concurrency int) error {
	et, err := s.newExportType("sourcecode.commit")
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

			batch := []rpcdef.ExportObj{}
			for _, commit := range res {
				c2 := sourcecode.Commit{}
				c2.CustomerID = s.customerID
				c2.RefType = "sourcecode.commit"
				c2.RefID = commit.CommitHash
				c2.RepoID = s.qc.RepoID(repoID)
				//c2.Branch = branchName
				c2.AuthorRefID = s.qc.UserID(commit.AuthorRefID)
				c2.CommitterRefID = s.qc.UserID(commit.CommitterRefID)
				batch = append(batch, rpcdef.ExportObj{Data: c2.ToMap()})

			}
			return pi, et.Send(batch)
		})
}
