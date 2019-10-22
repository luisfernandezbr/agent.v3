package main

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/pinpt/agent.next/integrations/pkg/objsender"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/bitbucket/api"
	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsForRepo(logger hclog.Logger, repo commonrepo.Repo,
	pullRequestSender *objsender.Session,
	commitsSender *objsender.Session) (rerr error) {

	logger = logger.With("repo", repo.NameWithOwner)
	logger.Info("exporting")

	// export changed pull requests
	var pullRequestsErr error
	pullRequestsInitial := make(chan []sourcecode.PullRequest)
	go func() {
		defer close(pullRequestsInitial)
		if err := s.exportPullRequestsRepo(logger, repo, pullRequestSender, pullRequestsInitial, pullRequestSender.LastProcessedTime()); err != nil {
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
		err := s.exportPullRequestsComments(logger, pullRequestSender, repo, pullRequestsForComments)
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
				commits, err := s.exportPullRequestCommits(logger, repo, pr.RefID)
				if err != nil {
					setErr(fmt.Errorf("error getting commits %s", err))
					return
				}

				for _, c := range commits {
					pr.CommitShas = append(pr.CommitShas, c.Sha)
				}

				pr.CommitIds = s.qc.IDs.CodeCommits(pr.RepoID, pr.CommitShas)
				if len(pr.CommitShas) == 0 {
					logger.Info("found PullRequest with no commits (ignoring it)", "repo", repo.NameWithOwner, "pr_ref_id", pr.RefID, "pr.url", pr.URL)
				} else {
					pr.BranchID = s.qc.IDs.CodeBranch(pr.RepoID, pr.BranchName, pr.CommitShas[0])
				}

				if err = pullRequestSender.Send(&pr); err != nil {
					setErr(err)
					return
				}

				for _, c := range commits {
					c.BranchID = pr.BranchID
					err := commitsSender.Send(c)
					if err != nil {
						setErr(err)
						return
					}
				}
			}
		}
	}()
	wg.Wait()
	return
}

func (s *Integration) exportPullRequestsRepo(logger hclog.Logger, repo commonrepo.Repo, sender *objsender.Session, pullRequests chan []sourcecode.PullRequest, lastProcessed time.Time) error {
	return api.PaginateNewerThan(logger, lastProcessed, func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (api.PageInfo, error) {
		pi, res, err := api.PullRequestPage(s.qc, sender, repo.ID, repo.NameWithOwner, parameters, stopOnUpdatedAt)
		if err != nil {
			return pi, err
		}
		if err = sender.SetTotal(pi.Total); err != nil {
			return pi, err
		}
		pullRequests <- res
		return pi, nil
	})
}
