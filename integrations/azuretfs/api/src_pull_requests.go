package api

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/pinpt/agent.next/integrations/pkg/objsender"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchPullRequests calls the pull request api and processes the reponse sending each object to the corresponding channel async
// sourcecode.PullRequest, sourcecode.PullRequestReview, sourcecode.PullRequestComment, and sourcecode.PullRequestCommit
func (api *API) FetchPullRequests(repoid string, reponame string, sender *objsender.Session) ([]rpcdef.GitRepoFetchPR, error) {
	res, err := api.fetchPullRequests(repoid)
	if err != nil {
		return nil, err
	}
	fromdate := sender.LastProcessedTime()
	incremental := !fromdate.IsZero()
	repoRefID := api.IDs.CodeRepo(repoid)

	var pullrequests []pullRequestResponse
	var pullrequestcomments []pullRequestResponse
	var fetchprs []rpcdef.GitRepoFetchPR
	var fetchprsMutex sync.Mutex
	for _, p := range res {
		// if this is not incremental, return only the objects created after the fromdate
		if !incremental || p.CreationDate.After(fromdate) {
			pullrequests = append(pullrequests, p)
		}
		// if this is not incremental, only fetch the comments if this pr is still opened or was closed after the last processed date
		if !incremental || (p.Status == "active" || (incremental && p.ClosedDate.After(fromdate))) {
			pullrequestcomments = append(pullrequestcomments, p)
		}
	}

	prsender, err := sender.Session(sourcecode.PullRequestModelName.String(), repoid, reponame)
	if err != nil {
		api.logger.Error("error creating sender session for pull request", "err", err, "repo_id", repoid, "repo_name", reponame)
		return nil, err
	}
	if err := prsender.SetTotal(len(pullrequests)); err != nil {
		api.logger.Error("error setting total pullrequests on FetchPullRequests", "err", err)
	}
	async := NewAsync(api.concurrency)
	for _, p := range pullrequests {
		pr := pullRequestResponseWithShas{}
		pr.pullRequestResponse = p
		pr.SourceBranch = strings.TrimPrefix(p.SourceBranch, "refs/heads/")
		pr.TargetBranch = strings.TrimPrefix(p.TargetBranch, "refs/heads/")
		async.Do(func() {
			commits, err := api.fetchPullRequestCommits(pr.Repository.ID, pr.PullRequestID)
			if err != nil {
				api.logger.Error("error fetching commits for PR, skipping", "pr_id", pr.PullRequestID, "repo_id", pr.Repository.ID, "err", err)
				return
			}
			if len(commits) == 0 {
				return
			}
			pridstring := fmt.Sprintf("%d", pr.PullRequestID)
			prcsender, err := prsender.Session(sourcecode.PullRequestCommitModelName.String(), pridstring, pridstring)
			if err != nil {
				api.logger.Error("error creating sender session for pull request commits", "pr_id", pr.PullRequestID, "repo_id", pr.Repository.ID, "err", err)
				return
			}
			if err := prcsender.SetTotal(len(commits)); err != nil {
				api.logger.Error("error setting total pull request commits on FetchPullRequests", "err", err)
			}
			for _, commit := range commits {
				pr.commitshas = append(pr.commitshas, commit.CommitID)
				pr := pr
				api.sendPullRequestCommitObjects(repoRefID, pr, prcsender)
			}
			if err := prcsender.Done(); err != nil {
				api.logger.Error("error with sender done in pull request commits", "err", err)
			}
			prrsender, err := prsender.Session(sourcecode.PullRequestReviewModelName.String(), pridstring, pridstring)
			if err != nil {
				api.logger.Error("error creating sender session for pull request reviews", "pr_id", pr.PullRequestID, "repo_id", pr.Repository.ID, "err", err)
				return
			}
			if err := prrsender.SetTotal(len(pr.Reviewers)); err != nil {
				api.logger.Error("error setting total pull request reviews on FetchPullRequests", "err", err)
			}
			fetchprsMutex.Lock()
			fetchprs = append(fetchprs, rpcdef.GitRepoFetchPR{
				ID:            api.IDs.CodePullRequest(repoRefID, pridstring),
				RefID:         pridstring,
				URL:           pr.URL,
				BranchName:    pr.SourceBranch,
				LastCommitSHA: pr.commitshas[len(pr.commitshas)-1],
			})
			fetchprsMutex.Unlock()

			api.sendPullRequestObjects(repoRefID, pr, prsender, prrsender)
			if err := prrsender.Done(); err != nil {
				api.logger.Error("error with sender done in pull request reviews", "err", err)

			}
		})
	}

	for _, pr := range pullrequestcomments {
		pr := pr
		async.Do(func() {
			prcsender, err := prsender.Session(sourcecode.PullRequestCommentModelName.String(), fmt.Sprintf("%d", pr.PullRequestID), pr.Title)
			if err != nil {
				api.logger.Error("error creating sender session for pull request comments", "pr_id", pr.PullRequestID, "repo_id", repoid, "err", err)
				return
			}
			api.sendPullRequestCommentObject(repoRefID, pr, prcsender)
			if err := prcsender.Done(); err != nil {
				api.logger.Error("error with sender done in pull request comments", "err", err)
			}
		})
	}
	async.Wait()

	return fetchprs, prsender.Done()
}

func (api *API) sendPullRequestCommentObject(repoRefID string, pr pullRequestResponse, sender *objsender.Session) {
	comments, err := api.fetchPullRequestComments(pr.Repository.ID, pr.PullRequestID)
	if err != nil {
		api.logger.Error("error fetching comments for PR, skipping", "pr_id", pr.PullRequestID, "repo_id", pr.Repository.ID, "err", err)
		return
	}
	var total []*sourcecode.PullRequestComment
	for _, cm := range comments {
		for _, e := range cm.Comments {
			// comment type "text" means it's a real user instead of system
			if e.CommentType != "text" {
				continue
			}
			refid := fmt.Sprintf("%d_%d", cm.ID, e.ID)
			c := &sourcecode.PullRequestComment{
				Body:          e.Content,
				CustomerID:    api.customerid,
				PullRequestID: api.IDs.CodePullRequest(repoRefID, fmt.Sprintf("%d", pr.PullRequestID)),
				RefID:         refid,
				RefType:       api.reftype,
				RepoID:        repoRefID,
				UserRefID:     e.Author.ID,
			}
			date.ConvertToModel(e.PublishedDate, &c.CreatedDate)
			date.ConvertToModel(e.LastUpdatedDate, &c.UpdatedDate)
			total = append(total, c)
		}
	}
	if err := sender.SetTotal(len(total)); err != nil {
		api.logger.Error("error setting total pull request comments on sendPullRequestCommentObject", "err", err)
	}
	for _, c := range total {
		if err := sender.Send(c); err != nil {
			api.logger.Error("error sending pull request comments", "id", c.RefID, "err", err)
		}
	}
}

func (api *API) sendPullRequestObjects(repoRefID string, p pullRequestResponseWithShas, prsender *objsender.Session, prrsender *objsender.Session) {

	pr := &sourcecode.PullRequest{
		BranchName:     p.SourceBranch,
		CreatedByRefID: p.CreatedBy.ID,
		CustomerID:     api.customerid,
		Description:    p.Description,
		RefID:          fmt.Sprintf("%d", p.PullRequestID),
		RefType:        api.reftype,
		RepoID:         repoRefID,
		Title:          p.Title,
		URL:            p.URL,
		CommitShas:     p.commitshas,
		Identifier:     fmt.Sprintf("#%d", p.PullRequestID), // format for displaying the PR in app
	}
	if p.commitshas != nil {
		pr.BranchID = api.IDs.CodeBranch(repoRefID, p.SourceBranch, p.commitshas[0])
		pr.CommitIds = api.IDs.CodeCommits(repoRefID, p.commitshas)
	}

	switch p.Status {
	case "completed":
		pr.Status = sourcecode.PullRequestStatusMerged
		pr.MergeSha = p.LastMergeCommit.CommidID
		pr.MergeCommitID = ids.CodeCommit(api.customerid, api.reftype, repoRefID, pr.MergeSha)
		date.ConvertToModel(p.CompletionQueueTime, &pr.MergedDate)
	case "active":
		pr.Status = sourcecode.PullRequestStatusOpen
	case "abandoned":
		pr.Status = sourcecode.PullRequestStatusClosed
	}
	for i, r := range p.Reviewers {
		var state sourcecode.PullRequestReviewState
		switch r.Vote {
		case -10:
			pr.ClosedByRefID = r.ID
			state = sourcecode.PullRequestReviewStateDismissed
		case -5:
			state = sourcecode.PullRequestReviewStateChangesRequested
		case 0:
			state = sourcecode.PullRequestReviewStatePending
		case 5:
			state = sourcecode.PullRequestReviewStateCommented
		case 10:
			if pr.ClosedByRefID == "" {
				pr.ClosedByRefID = r.ID
			}
			pr.MergedByRefID = r.ID
			state = sourcecode.PullRequestReviewStateApproved
		}
		refid := hash.Values(i, p.PullRequestID, r.ID)
		if err := prrsender.Send(&sourcecode.PullRequestReview{
			CustomerID:    api.customerid,
			PullRequestID: api.IDs.CodePullRequest(repoRefID, fmt.Sprintf("%d", p.PullRequestID)),
			RefID:         refid,
			RefType:       api.reftype,
			RepoID:        repoRefID,
			State:         state,
			URL:           r.URL,
			UserRefID:     r.ID,
		}); err != nil {
			api.logger.Error("error sending pull request review", "id", pr.RefID, "err", err)
		}
	}
	date.ConvertToModel(p.ClosedDate, &pr.ClosedDate)
	date.ConvertToModel(p.CreationDate, &pr.CreatedDate)
	if err := prsender.Send(pr); err != nil {
		api.logger.Error("error sending pull request", "id", pr.RefID, "err", err)
	}
}
func (api *API) sendPullRequestCommitObjects(repoRefID string, p pullRequestResponseWithShas, sender *objsender.Session) error {
	sha := p.commitshas[len(p.commitshas)-1]
	commits, err := api.fetchSingleCommit(p.Repository.ID, sha)
	if err != nil {
		return err
	}
	for _, c := range commits {
		commit := &sourcecode.PullRequestCommit{
			Additions: c.ChangeCounts.Add,
			// AuthorRefID: not provided
			BranchID: api.IDs.CodeBranch(repoRefID, p.SourceBranch, p.commitshas[0]),
			// CommitterRefID: not provided
			CustomerID:    api.customerid,
			Deletions:     c.ChangeCounts.Delete,
			Message:       c.Comment,
			PullRequestID: api.IDs.CodePullRequest(repoRefID, fmt.Sprintf("%d", p.PullRequestID)),
			RefID:         sha,
			RefType:       api.reftype,
			RepoID:        repoRefID,
			Sha:           sha,
			URL:           c.RemoteURL,
		}
		date.ConvertToModel(c.Push.Date, &commit.CreatedDate)
		if err := sender.Send(commit); err != nil {
			api.logger.Error("error sending pull request commit", "id", commit.RefID, "err", err)
		}
	}
	return nil
}

func (api *API) fetchPullRequests(repoid string) ([]pullRequestResponse, error) {
	u := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests`, url.PathEscape(repoid))
	var res []pullRequestResponse
	if err := api.getRequest(u, stringmap{"status": "all", "$top": "1000"}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *API) fetchPullRequestComments(repoid string, prid int64) ([]commentsReponse, error) {
	u := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/threads`, url.PathEscape(repoid), prid)
	var res []commentsReponse
	if err := api.getRequest(u, nil, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *API) fetchPullRequestCommits(repoid string, prid int64) ([]commitsResponseLight, error) {
	u := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/commits`, url.PathEscape(repoid), prid)
	var res []commitsResponseLight
	// there's a bug with paging this api in azure
	if err := api.getRequest(u, stringmap{"$top": "1000"}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *API) fetchSingleCommit(repoid string, commitid string) ([]singleCommitResponse, error) {
	u := fmt.Sprintf(`_apis/git/repositories/%s/commits/%s`, url.PathEscape(repoid), url.PathEscape(commitid))
	var res []singleCommitResponse
	if err := api.getRequest(u, stringmap{
		"changeCount": "1000",
	}, &res); err != nil {
		return nil, err
	}
	return res, nil
}
