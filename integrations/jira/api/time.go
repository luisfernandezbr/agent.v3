package api

import (
	"time"

	"github.com/pinpt/go-datamodel/work"
)

func TimeIssueCreated(ts time.Time) (res work.IssueCreated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimeIssueDueDate(ts time.Time) (res work.IssueDueDate) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimeIssueUpdated(ts time.Time) (res work.IssueUpdated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimeChangelogCreated(ts time.Time) (res work.ChangelogCreated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimeSprintStarted(ts time.Time) (res work.SprintStarted) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimeSprintEnded(ts time.Time) (res work.SprintEnded) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimeSprintCompleted(ts time.Time) (res work.SprintCompleted) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func TimeSprintFetched(ts time.Time) (res work.SprintFetched) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}
