package main

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/bitbucket/api"
	"github.com/pinpt/agent.next/pkg/commonrepo"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsForRepo(logger hclog.Logger, repo commonrepo.Repo,
	pullRequestSender *objsender.IncrementalDateBased,
	commentsSender *objsender.NotIncremental,
	reviewSender *objsender.NotIncremental) (rerr error) {

	logger = logger.With("repo", repo.NameWithOwner)
	logger.Info("exporting")

	// export changed pull requests
	var pullRequestsErr error
	pullRequestsInitial := make(chan []sourcecode.PullRequest)
	go func() {
		defer close(pullRequestsInitial)
		err := s.exportPullRequestsRepo(logger, repo, pullRequestsInitial, pullRequestSender.LastProcessed, s.pullRequestReviewsSender)
		if err != nil {
			pullRequestsErr = err
		}
	}()

	// export comments, reviews, commits concurrently
	pullRequestsForComments := make(chan []sourcecode.PullRequest, 10)
	pullRequestsForCommits := make(chan []sourcecode.PullRequest, 10)

	var errMu sync.Mutex
	setErr := func(err error) {
		logger.Error("failed repo export", "e", err)
		errMu.Lock()
		defer errMu.Unlock()
		if rerr == nil {
			rerr = err
		}
		// drain all pull requests on error
		for range pullRequestsForComments {
		}
		for range pullRequestsForCommits {
		}
	}

	go func() {
		for item := range pullRequestsInitial {
			pullRequestsForComments <- item
			pullRequestsForCommits <- item
		}
		close(pullRequestsForComments)
		close(pullRequestsForCommits)

		if pullRequestsErr != nil {
			setErr(pullRequestsErr)
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.exportPullRequestsComments(logger, commentsSender, repo, pullRequestsForComments)
		if err != nil {
			setErr(fmt.Errorf("error getting comments %s", err))
		}
	}()

	// set commits on the rp and then send the pr
	wg.Add(1)
	go func() {
		defer wg.Done()
		for prs := range pullRequestsForCommits {
			for _, pr := range prs {
				commits, err := s.exportPullRequestCommits(logger, repo.NameWithOwner, pr.RefID)
				if err != nil {
					setErr(fmt.Errorf("error getting commits %s", err))
					return
				}
				pr.CommitShas = commits
				pr.CommitIds = ids.CodeCommits(s.qc.CustomerID, s.refType, pr.RepoID, commits)
				if len(pr.CommitShas) == 0 {
					logger.Info("found PullRequest with no commits (ignoring it)", "repo", repo.NameWithOwner, "pr_ref_id", pr.RefID, "pr.url", pr.URL)
				} else {
					pr.BranchID = s.qc.BasicInfo.BranchID(pr.RepoID, pr.BranchName, pr.CommitShas[0])
				}
				err = pullRequestSender.SendMap(pr.ToMap())
				if err != nil {
					setErr(err)
					return
				}
			}
		}
	}()
	wg.Wait()
	return
}

func (s *Integration) exportPullRequestsRepo(logger hclog.Logger, repo commonrepo.Repo, pullRequests chan []sourcecode.PullRequest, lastProcessed time.Time, reviewsSender *objsender.NotIncremental) error {
	return api.PaginateNewerThan(logger, lastProcessed, func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestPage(s.qc, repo.ID, repo.NameWithOwner, parameters, stopOnUpdatedAt, reviewsSender)
		if err != nil {
			return pi, err
		}
		pullRequests <- res
		return pi, nil
	})
}
