package api

import (
	"time"

	"github.com/pinpt/go-datamodel/sourcecode"
)

func TimePullRequestClosed(ts time.Time) (res sourcecode.PullRequestClosed) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimePullRequestCreated(ts time.Time) (res sourcecode.PullRequestCreated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimePullRequestMerged(ts time.Time) (res sourcecode.PullRequestMerged) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimePullRequestUpdated(ts time.Time) (res sourcecode.PullRequestUpdated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimePullRequestCommentCreated(ts time.Time) (res sourcecode.PullRequestCommentCreated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimePullRequestCommentUpdated(ts time.Time) (res sourcecode.PullRequestCommentUpdated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimePullRequestReviewCreated(ts time.Time) (res sourcecode.PullRequestReviewCreated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}
