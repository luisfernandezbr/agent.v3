package azureapi

import (
	"fmt"
	purl "net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// FetchPullRequests calls the pull request api and processes the reponse sending each object to the corresponding channel async
// sourcecode.PullRequest, sourcecode.PullRequestReview, sourcecode.PullRequestComment, and sourcecode.PullRequestCommit
func (api *API) FetchPullRequests(repoid string, fromdate time.Time, prchan chan<- datamodel.Model, prrchan chan<- datamodel.Model, prcchan chan<- datamodel.Model, prcmchan chan<- datamodel.Model) error {
	res, err := api.fetchPullRequests(repoid)
	if err != nil {
		return err
	}
	incremental := !fromdate.IsZero()
	a := NewAsync(5)
	for _, p := range res {
		// if this is not incremental, return only the objects created after the fromdate
		if !incremental || p.CreationDate.After(fromdate) {
			commits, err := api.fetchPullRequestCommits(repoid, p.PullRequestID)
			if err != nil {
				api.logger.Error("error fetching commits for PR, skiping", "pr-id", p.PullRequestID, "repo-id", repoid)
				continue
			}
			for _, commit := range commits {
				p.commitshas = append(p.commitshas, commit.CommitID)
				a.Send(AsyncMessage{
					Data: p,
					Func: func(data interface{}) {
						p := data.(pullRequestResponse)
						api.sendPullRequestCommitObjects(repoid, p, prcmchan)
					},
				})

			}
			a.Send(AsyncMessage{
				Data: p,
				Func: func(data interface{}) {
					p := data.(pullRequestResponse)
					api.sendPullRequestObjects(repoid, p, prchan, prrchan, prcmchan)
				},
			})
		}
		// if this is not incremental, only fetch the comments if this pr is still opened or was closed after the last processed date
		if !incremental || (p.Status == "active" || (incremental && p.ClosedDate.After(fromdate))) {
			a.Send(AsyncMessage{
				Data: p,
				Func: func(data interface{}) {
					p := data.(pullRequestResponse)
					api.sendPullRequestCommentObject(repoid, p.PullRequestID, prcchan)
				},
			})
		}
	}
	a.Wait()
	return nil
}

func (api *API) sendPullRequestCommentObject(repoid string, prid int64, prcchan chan<- datamodel.Model) {
	comments, err := api.fetchPullRequestComments(repoid, prid)
	if err != nil {
		api.logger.Error("error fetching comments for PR, skiping", "pr-id", prid, "repo-id", repoid)
		return
	}
	for _, cm := range comments {
		for _, e := range cm.Comments {
			// comment type "text" means it's a real user instead of system
			if e.CommentType != "text" {
				continue
			}
			refid := fmt.Sprintf("%d_%d", cm.ID, e.ID)
			c := sourcecode.PullRequestComment{
				Body:          e.Content,
				CustomerID:    api.customerid,
				PullRequestID: api.PullRequestID(fmt.Sprintf("%d", prid), refid),
				RefID:         refid,
				RefType:       api.reftype,
				RepoID:        api.RepoID(repoid),
				UserRefID:     e.Author.ID,
			}
			date.ConvertToModel(e.PublishedDate, &c.CreatedDate)
			date.ConvertToModel(e.LastUpdatedDate, &c.UpdatedDate)
			prcchan <- &c
		}
	}
}

func (api *API) sendPullRequestObjects(repoid string, p pullRequestResponse, prchan chan<- datamodel.Model, prrchan chan<- datamodel.Model, prcmchan chan<- datamodel.Model) {
	prid := p.PullRequestID

	pr := sourcecode.PullRequest{
		BranchName:     p.SourceBranch,
		CreatedByRefID: p.CreatedBy.ID,
		CustomerID:     api.customerid,
		Description:    p.Description,
		RefID:          fmt.Sprintf("%d", prid),
		RefType:        api.reftype,
		RepoID:         api.RepoID(p.Repository.ID),
		Title:          p.Title,
		URL:            p.URL,
		CommitShas:     p.commitshas,
	}
	if p.commitshas != nil {
		pr.BranchID = api.BranchID(repoid, p.SourceBranch, p.commitshas[0])
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
		refid := hash.Values(i, prid, r.ID)
		prrchan <- &sourcecode.PullRequestReview{
			CustomerID:    api.customerid,
			PullRequestID: api.PullRequestID(fmt.Sprintf("%d", prid), refid),
			RefID:         refid,
			RefType:       api.reftype,
			RepoID:        api.RepoID(repoid),
			State:         state,
			URL:           r.URL,
			UserRefID:     r.ID,
		}
	}
	date.ConvertToModel(p.ClosedDate, &pr.ClosedDate)
	date.ConvertToModel(p.CreationDate, &pr.CreatedDate)
	prchan <- &pr
}
func (api *API) sendPullRequestCommitObjects(repoid string, p pullRequestResponse, commichan chan<- datamodel.Model) error {
	sha := p.commitshas[len(p.commitshas)-1]
	commits, err := api.fetchSingleCommit(repoid, sha)
	if err != nil {
		return err
	}
	for _, c := range commits {
		commit := &sourcecode.PullRequestCommit{
			Additions: c.ChangeCounts.Add,
			// AuthorRefID: not provided
			BranchID: api.BranchID(repoid, p.SourceBranch, p.commitshas[0]),
			// CommitterRefID: not provided
			CustomerID:    api.customerid,
			Deletions:     c.ChangeCounts.Delete,
			Message:       c.Comment,
			PullRequestID: api.PullRequestID(p.Repository.ID, sha),
			RefID:         sha,
			RefType:       api.reftype,
			RepoID:        p.Repository.ID,
			Sha:           sha,
			URL:           c.RemoteURL,
		}
		date.ConvertToModel(c.Push.Date, &commit.CreatedDate)
		commichan <- commit

	}
	return nil
}

func (api *API) fetchPullRequests(repoid string) ([]pullRequestResponse, error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests`, purl.PathEscape(repoid))
	var res []pullRequestResponse
	if err := api.getRequest(url, stringmap{"status": "all", "$top": "1000"}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *API) fetchPullRequestComments(repoid string, prid int64) ([]commentsReponse, error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/threads`, purl.PathEscape(repoid), prid)
	var res []commentsReponse
	if err := api.getRequest(url, nil, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *API) fetchPullRequestCommits(repoid string, prid int64) ([]commitsResponseLight, error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%d/commits`, purl.PathEscape(repoid), prid)
	var res []commitsResponseLight
	// there's a bug with paging this api in azure
	if err := api.getRequest(url, stringmap{"$top": "1000"}, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *API) fetchSingleCommit(repoid string, commitid string) ([]singleCommitResponse, error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/commits/%s`, purl.PathEscape(repoid), purl.PathEscape(commitid))
	var res []singleCommitResponse
	if err := api.getRequest(url, stringmap{
		"changeCount": "1000",
	}, &res); err != nil {
		return nil, err
	}
	return res, nil
}
