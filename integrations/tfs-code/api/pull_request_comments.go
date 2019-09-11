package api

import (
	"fmt"
	purl "net/url"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type commentsReponse struct {
	Comments []struct {
		Author struct {
			ID string `json:"id"`
		} `json:"author"`
		CommentType     string `json:"commentType"`
		Content         string `json:"content"`
		ID              int64  `json:"id"`
		LastUpdatedDate string `json:"lastUpdatedDate"`
		PublishedDate   string `json:"publishedDate"`
	} `json:"comments"`
	ID int64 `json:"id"`
}

// FetchPullRequestComments calls the pull request comment api returns a list of sourcecode.PullRequestComment
func (a *TFSAPI) FetchPullRequestComments(repoid string, prid string) (cmts []*sourcecode.PullRequestComment, err error) {
	url := fmt.Sprintf(`_apis/git/repositories/%s/pullRequests/%s/threads`, purl.PathEscape(repoid), prid)
	var res []commentsReponse
	if err = a.doRequest(url, nil, time.Time{}, &res); err != nil {
		return
	}
	for _, cm := range res {
		for _, e := range cm.Comments {
			if e.CommentType != "text" {
				continue
			}
			refid := fmt.Sprintf("%d_%d", cm.ID, e.ID)
			c := &sourcecode.PullRequestComment{
				Body:          e.Content,
				PullRequestID: a.PullRequestID(prid, refid),
				RefID:         refid,
				RefType:       a.reftype,
				CustomerID:    a.customerid,
				RepoID:        a.RepoID(repoid),
				UserRefID:     e.Author.ID,
			}

			if d, err := datetime.ISODateToTime(e.PublishedDate); err != nil {
				a.logger.Warn("error converting date in tfs-code FetchPullRequestComments 1", "raw date", e.PublishedDate, "err", err)
			} else {
				date.ConvertToModel(d, &c.CreatedDate)
			}
			if d, err := datetime.ISODateToTime(e.LastUpdatedDate); err != nil {
				a.logger.Warn("error converting date in tfs-code FetchPullRequestComments 2", "raw date", e.LastUpdatedDate, "err", err)
			} else {
				date.ConvertToModel(d, &c.UpdatedDate)
			}
			cmts = append(cmts, c)
		}
	}
	return
}
