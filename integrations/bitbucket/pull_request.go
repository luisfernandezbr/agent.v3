package main

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/rpcdef"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/bitbucket/api"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportPullRequestsForRepo(ctx *repoprojects.ProjectCtx, repo commonrepo.Repo) (res []rpcdef.GitRepoFetchPR, rerr error) {

	pullRequestSender, err := ctx.Session(sourcecode.PullRequestModelName)
	if err != nil {
		rerr = err
		return
	}

	commentsSender, err := ctx.Session(sourcecode.PullRequestCommentModelName)
	if err != nil {
		rerr = err
		return
	}

	commitsSender, err := ctx.Session(sourcecode.PullRequestCommitModelName)
	if err != nil {
		rerr = err
		return
	}

	reviewsSender, err := ctx.Session(sourcecode.PullRequestReviewModelName)
	if err != nil {
		rerr = err
		return
	}

	ctx.Logger.Info("exporting")

	// export changed pull requests
	var pullRequestsErr error
	pullRequestsInitial := make(chan []sourcecode.PullRequest)
	go func() {
		defer close(pullRequestsInitial)
		if err := s.exportPullRequestsRepo(ctx.Logger, repo, pullRequestSender, reviewsSender, pullRequestsInitial, pullRequestSender.LastProcessedTime()); err != nil {
			pullRequestsErr = err
		}
	}()

	// export comments, reviews, commits concurrently
	pullRequestsForComments := make(chan []sourcecode.PullRequest, 10)
	pullRequestsForCommits := make(chan []sourcecode.PullRequest, 10)

	var errMu sync.Mutex
	setErr := func(err error) {
		ctx.Logger.Error("failed repo export", "e", err)
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
		err := s.exportPullRequestsComments(ctx.Logger, commentsSender, repo, pullRequestsForComments)
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
				logger := ctx.Logger.With("pr_id", pr.RefID)
				commits, err := s.exportPullRequestCommits(logger, repo, pr, commitsSender)
				if err != nil {
					setErr(fmt.Errorf("error getting commits %s", err))
					return
				}

				if len(commits) > 0 {
					meta := rpcdef.GitRepoFetchPR{}
					repoID := s.qc.IDs.CodeRepo(repo.RefID)
					meta.ID = s.qc.IDs.CodePullRequest(repoID, pr.RefID)
					meta.RefID = pr.RefID
					meta.URL = pr.URL
					meta.BranchName = pr.BranchName
					meta.LastCommitSHA = commits[0].Sha
					res = append(res, meta)
				}
				for ind := len(commits) - 1; ind >= 0; ind-- {
					pr.CommitShas = append(pr.CommitShas, commits[ind].Sha)
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

func (s *Integration) exportPullRequestsRepo(logger hclog.Logger, repo commonrepo.Repo, prSender *objsender.Session, reviewsSender *objsender.Session, pullRequests chan []sourcecode.PullRequest, lastProcessed time.Time) error {

	params := url.Values{}
	params.Add("state", "MERGED")
	params.Add("state", "SUPERSEDED")
	params.Add("state", "OPEN")
	params.Add("state", "DECLINED")
	// Greater than 50 throws "Invalid pagelen"
	params.Set("pagelen", "50")

	stopOnUpdatedAt := prSender.LastProcessedTime()
	if !stopOnUpdatedAt.IsZero() {
		params.Set("q", fmt.Sprintf(" updated_on > %s", stopOnUpdatedAt.UTC().Format("2006-01-02T15:04:05.000000-07:00")))
	}

	return api.Paginate(func(nextPage api.NextPage) (api.NextPage, error) {
		pi, res, err := api.PullRequestPage(s.qc, logger, reviewsSender, repo, params, nextPage)
		if err != nil {
			return pi, err
		}
		pullRequests <- res
		return pi, nil
	})
}
