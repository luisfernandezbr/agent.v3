package api

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchPullRequests calls the pull request api and processes the reponse sending each object to the corresponding channel async
// sourcecode.PullRequest, sourcecode.PullRequestReview, sourcecode.PullRequestComment, and sourcecode.PullRequestCommit
func (api *API) FetchPullRequests(ctx *repoprojects.ProjectCtx, repoid string, reponame string, repoSender *objsender.Session) (_ []rpcdef.GitRepoFetchPR, rerr error) {
	res, err := api.fetchPullRequests(repoid)
	if err != nil {
		rerr = err
		return
	}
	fromdate := repoSender.LastProcessedTime()
	incremental := !fromdate.IsZero()
	repoRefID := api.IDs.CodeRepo(repoid)

	var pullrequests []pullRequestResponse
	var pullrequestcomments []pullRequestResponse
	var fetchprs []rpcdef.GitRepoFetchPR
	for _, p := range res {
		// modify the url to show the ui instead of api call
		p.URL = strings.ToLower(p.URL)
		p.URL = strings.Replace(p.URL, "_apis/git/repositories", "_git", 1)
		p.URL = strings.Replace(p.URL, "/pullrequests/", "/pullrequest/", 1)

		// if this is not incremental, return only the objects created after the fromdate
		if !incremental || p.CreationDate.After(fromdate) {
			pullrequests = append(pullrequests, p)
		}
		// if this is not incremental, only fetch the comments if this pr is still opened or was closed after the last processed date
		if !incremental || (p.Status == "active" || (incremental && p.ClosedDate.After(fromdate))) {
			pullrequestcomments = append(pullrequestcomments, p)
		}
	}

	prsender, err := ctx.Session(sourcecode.PullRequestModelName)
	if err != nil {
		rerr = err
		return
	}
	if err := prsender.SetTotal(len(pullrequests)); err != nil {
		rerr = err
		return
	}

	for _, p := range pullrequests {
		pr := pullRequestResponseWithShas{}
		pr.pullRequestResponse = p
		pr.SourceBranch = strings.TrimPrefix(p.SourceBranch, "refs/heads/")
		pr.TargetBranch = strings.TrimPrefix(p.TargetBranch, "refs/heads/")

		commits, err := api.fetchPullRequestCommits(pr.Repository.ID, pr.PullRequestID)
		if err != nil {
			rerr = fmt.Errorf("error fetching commits for PR, skipping pr_id:%v repo_id:%v err:%v", pr.PullRequestID, pr.Repository.ID, err)
			return
		}
		if len(commits) == 0 {
			return
		}

		pridstring := fmt.Sprintf("%d", pr.PullRequestID)
		prcsender, err := prsender.Session(sourcecode.PullRequestCommitModelName.String(), pridstring, pridstring)
		if err != nil {
			rerr = fmt.Errorf("error creating sender session for pull request commits pr_id:%v repo_id:%v err: %v", pr.PullRequestID, pr.Repository.ID, err)
			return
		}
		if err := prcsender.SetTotal(len(commits)); err != nil {
			rerr = err
			return
		}
		for _, commit := range commits {
			pr.commitshas = append(pr.commitshas, commit.CommitID)
			pr := pr
			api.sendPullRequestCommitObjects(repoRefID, pr, prcsender)
		}
		if err := prcsender.Done(); err != nil {
			rerr = err
			return
		}
		fetchprs = append(fetchprs, rpcdef.GitRepoFetchPR{
			ID:            api.IDs.CodePullRequest(repoRefID, pridstring),
			RefID:         pridstring,
			URL:           pr.URL,
			BranchName:    pr.SourceBranch,
			LastCommitSHA: pr.commitshas[len(pr.commitshas)-1],
		})
		api.sendPullRequestObjects(repoRefID, pr, reponame, prsender)
	}

	for _, pr := range pullrequestcomments {
		pridstring := fmt.Sprintf("%d", pr.PullRequestID)
		prcsender, err := prsender.Session(sourcecode.PullRequestCommentModelName.String(), pridstring, pr.Title)
		if err != nil {
			rerr = err
			return
		}

		prrsender, err := prsender.Session(sourcecode.PullRequestReviewModelName.String(), pridstring, pridstring)
		if err != nil {
			rerr = err
			return
		}
		api.sendPullRequestCommentObject(repoRefID, pr, prcsender, prrsender)
		if err := prcsender.Done(); err != nil {
			rerr = err
			return
		}
		if err := prrsender.Done(); err != nil {
			rerr = err
			return

		}
	}

	return fetchprs, nil
}

var pullRequestCommentVotedReg = regexp.MustCompile(`(.+?)( voted )(-10|-5|0|5|10.*)`)

func (api *API) sendPullRequestCommentObject(repoRefID string, pr pullRequestResponse, prcsender *objsender.Session, prrsender *objsender.Session) {
	threads, err := api.fetchPullRequestThreads(pr.Repository.ID, pr.PullRequestID)
	if err != nil {
		api.logger.Error("error fetching threads for PR, skipping", "pr_id", pr.PullRequestID, "repo_id", pr.Repository.ID, "err", err)
		return
	}
	var total []*sourcecode.PullRequestComment
	for _, thread := range threads {
		for _, comment := range thread.Comments {
			// comment type "text" means it's a real user instead of system
			if comment.CommentType == "text" {
				refid := fmt.Sprintf("%d_%d", thread.ID, comment.ID)
				c := &sourcecode.PullRequestComment{
					Body:          comment.Content,
					CustomerID:    api.customerid,
					PullRequestID: api.IDs.CodePullRequest(repoRefID, fmt.Sprintf("%d", pr.PullRequestID)),
					RefID:         refid,
					RefType:       api.reftype,
					RepoID:        repoRefID,
					UserRefID:     comment.Author.ID,
				}
				date.ConvertToModel(comment.PublishedDate, &c.CreatedDate)
				date.ConvertToModel(comment.LastUpdatedDate, &c.UpdatedDate)
				total = append(total, c)
				continue
			}

			if comment.CommentType == "system" {
				if found := pullRequestCommentVotedReg.FindAllStringSubmatch(comment.Content, -1); len(found) > 0 {
					vote := found[0][3]
					var state sourcecode.PullRequestReviewState
					switch vote {
					case "-10":
						state = sourcecode.PullRequestReviewStateDismissed
					case "-5":
						state = sourcecode.PullRequestReviewStateChangesRequested
					case "0":
						state = sourcecode.PullRequestReviewStatePending
					case "5":
						state = sourcecode.PullRequestReviewStateCommented
					case "10":
						state = sourcecode.PullRequestReviewStateApproved
					}
					refid := hash.Values(pr.PullRequestID, thread.ID, comment.ID)
					review := &sourcecode.PullRequestReview{
						CustomerID:    api.customerid,
						PullRequestID: api.IDs.CodePullRequest(repoRefID, fmt.Sprintf("%d", pr.PullRequestID)),
						RefID:         refid,
						RefType:       api.reftype,
						RepoID:        repoRefID,
						State:         state,
						URL:           pr.URL,
						UserRefID:     thread.Identities["1"].ID,
					}
					date.ConvertToModel(comment.PublishedDate, &review.CreatedDate)
					if err := prrsender.Send(review); err != nil {
						api.logger.Error("error sending pull request review", "thread id", thread.ID, "comment id", comment.ID, "err", err)
					}
				}
			}
		}
	}
	if err := prcsender.SetTotal(len(total)); err != nil {
		api.logger.Error("error setting total pull request threads on sendPullRequestCommentObject", "err", err)
	}
	for _, c := range total {
		if err := prcsender.Send(c); err != nil {
			api.logger.Error("error sending pull request threads", "id", c.RefID, "err", err)
		}
	}
}

func (api *API) sendPullRequestObjects(repoRefID string, p pullRequestResponseWithShas, reponame string, prsender *objsender.Session) {

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
		Identifier:     reponame, // format for displaying the PR in app
	}
	if p.commitshas != nil {
		pr.BranchID = api.IDs.CodeBranch(repoRefID, p.SourceBranch, p.commitshas[0])
		pr.CommitIds = api.IDs.CodeCommits(repoRefID, p.commitshas)
	}
	date.ConvertToModel(p.ClosedDate, &pr.ClosedDate)
	date.ConvertToModel(p.CreationDate, &pr.CreatedDate)

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
	for _, r := range p.Reviewers {
		switch r.Vote {
		case -10:
			pr.ClosedByRefID = r.ID
		case 10:
			if pr.ClosedByRefID == "" {
				pr.ClosedByRefID = r.ID
			}
			pr.MergedByRefID = r.ID
		}
	}

	pr.Labels = make([]string, 0)
	for _, p := range p.Labels {
		pr.Labels = append(pr.Labels, p.Name)
	}

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

func (api *API) fetchPullRequestThreads(repoid string, prid int64) ([]threadsReponse, error) {
	u := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/threads`, url.PathEscape(repoid), prid)
	var res []threadsReponse
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
