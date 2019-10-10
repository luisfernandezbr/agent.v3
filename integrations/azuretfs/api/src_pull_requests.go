package api

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/integrations/pkg/objsender2"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchPullRequests calls the pull request api and processes the reponse sending each object to the corresponding channel async
// sourcecode.PullRequest, sourcecode.PullRequestReview, sourcecode.PullRequestComment, and sourcecode.PullRequestCommit
func (api *API) FetchPullRequests(repoid string, reponame string, sender *objsender2.Session) error {
	res, err := api.fetchPullRequests(repoid)
	if err != nil {
		return err
	}
	fromdate := sender.LastProcessedTime()
	incremental := !fromdate.IsZero()
	var pullrequests []pullRequestResponse
	var pullrequestcomments []pullRequestResponse
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
	prsender.SetTotal(len(pullrequests))
	async := NewAsync(api.concurrency)
	for _, p := range pullrequests {
		pr := pullRequestResponseWithShas{}
		pr.pullRequestResponse = p
		async.Do(func() {
			commits, err := api.fetchPullRequestCommits(repoid, pr.PullRequestID)
			pridstring := fmt.Sprintf("%d", pr.PullRequestID)
			if err != nil {
				api.logger.Error("error fetching commits for PR, skiping", "pr-id", pr.PullRequestID, "repo-id", repoid)
				return
			}
			prcsender, err := prsender.Session(sourcecode.PullRequestCommitModelName.String(), pridstring, pridstring)
			if err != nil {
				api.logger.Error("error creating sender session for pull request commits", "pr-id", pr.PullRequestID, "repo-id", repoid)
				return
			}
			prcsender.SetTotal(len(commits))
			for _, commit := range commits {
				pr.commitshas = append(pr.commitshas, commit.CommitID)
				pr := pr
				api.sendPullRequestCommitObjects(repoid, pr, prcsender)
			}
			prcsender.Done()
			prrsender, err := prsender.Session(sourcecode.PullRequestReviewModelName.String(), pridstring, pridstring)
			if err != nil {
				api.logger.Error("error creating sender session for pull request reviews", "pr-id", pr.PullRequestID, "repo-id", repoid)
				return
			}
			prrsender.SetTotal(len(pr.Reviewers))
			api.sendPullRequestObjects(repoid, pr, prsender, prrsender)
			prrsender.Done()
		})
	}

	for _, pr := range pullrequestcomments {
		pr := pr
		async.Do(func() {
			prcsender, err := prsender.Session(sourcecode.PullRequestCommentModelName.String(), fmt.Sprintf("%d", pr.PullRequestID), pr.Title)
			if err != nil {
				api.logger.Error("error creating sender session for pull request comments", "pr-id", pr.PullRequestID, "repo-id", repoid)
				return
			}
			api.sendPullRequestCommentObject(repoid, pr.PullRequestID, prcsender)
			prcsender.Done()
		})
	}
	async.Wait()
	prsender.Done()
	return nil
}

func (api *API) sendPullRequestCommentObject(repoid string, prid int64, sender *objsender2.Session) {
	comments, err := api.fetchPullRequestComments(repoid, prid)
	if err != nil {
		api.logger.Error("error fetching comments for PR, skiping", "pr-id", prid, "repo-id", repoid)
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
				PullRequestID: api.IDs.CodePullRequest(fmt.Sprintf("%d", prid), refid),
				RefID:         refid,
				RefType:       api.reftype,
				RepoID:        api.IDs.CodeRepo(repoid),
				UserRefID:     e.Author.ID,
			}
			date.ConvertToModel(e.PublishedDate, &c.CreatedDate)
			date.ConvertToModel(e.LastUpdatedDate, &c.UpdatedDate)
			total = append(total, c)
		}
	}
	sender.SetTotal(len(total))
	for _, c := range total {
		sender.Send(c)
	}
}

func (api *API) sendPullRequestObjects(repoid string, p pullRequestResponseWithShas, prsender *objsender2.Session, prrsender *objsender2.Session) {

	pr := &sourcecode.PullRequest{
		BranchName:     p.SourceBranch,
		CreatedByRefID: p.CreatedBy.ID,
		CustomerID:     api.customerid,
		Description:    p.Description,
		RefID:          fmt.Sprintf("%d", p.PullRequestID),
		RefType:        api.reftype,
		RepoID:         api.IDs.CodeRepo(p.Repository.ID),
		Title:          p.Title,
		URL:            p.URL,
		CommitShas:     p.commitshas,
	}
	if p.commitshas != nil {
		pr.BranchID = api.IDs.CodeBranch(repoid, p.SourceBranch, p.commitshas[0])
		pr.CommitIds = ids.CodeCommits(api.customerid, api.reftype, pr.RepoID, p.commitshas)
	}

	switch p.Status {
	case "completed":
		pr.Status = sourcecode.PullRequestStatusMerged
		pr.MergeSha = p.LastMergeCommit.CommidID
		pr.MergeCommitID = ids.CodeCommit(api.customerid, api.reftype, pr.RepoID, pr.MergeSha)
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
		prrsender.Send(&sourcecode.PullRequestReview{
			CustomerID:    api.customerid,
			PullRequestID: api.IDs.CodePullRequest(fmt.Sprintf("%d", p.PullRequestID), refid),
			RefID:         refid,
			RefType:       api.reftype,
			RepoID:        api.IDs.CodeRepo(repoid),
			State:         state,
			URL:           r.URL,
			UserRefID:     r.ID,
		})
	}
	date.ConvertToModel(p.ClosedDate, &pr.ClosedDate)
	date.ConvertToModel(p.CreationDate, &pr.CreatedDate)
	prsender.Send(pr)
}
func (api *API) sendPullRequestCommitObjects(repoid string, p pullRequestResponseWithShas, sender *objsender2.Session) error {
	sha := p.commitshas[len(p.commitshas)-1]
	commits, err := api.fetchSingleCommit(repoid, sha)
	if err != nil {
		return err
	}
	for _, c := range commits {
		commit := &sourcecode.PullRequestCommit{
			Additions: c.ChangeCounts.Add,
			// AuthorRefID: not provided
			BranchID: api.IDs.CodeBranch(repoid, p.SourceBranch, p.commitshas[0]),
			// CommitterRefID: not provided
			CustomerID:    api.customerid,
			Deletions:     c.ChangeCounts.Delete,
			Message:       c.Comment,
			PullRequestID: api.IDs.CodePullRequest(p.Repository.ID, sha),
			RefID:         sha,
			RefType:       api.reftype,
			RepoID:        p.Repository.ID,
			Sha:           sha,
			URL:           c.RemoteURL,
		}
		date.ConvertToModel(c.Push.Date, &commit.CreatedDate)
		sender.Send(commit)

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
