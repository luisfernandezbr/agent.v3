package exportrepo

import (
	"time"

	"github.com/pinpt/go-datamodel/sourcecode"
)

func timeCommitCreated(ts time.Time) (res sourcecode.CommitCreated) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}

func timeBlameDate(ts time.Time) (res sourcecode.BlameDate) {
	res.Rfc3339 = ts.Format(time.RFC3339)
	res.Epoch = ts.Unix()
	_, offset := ts.Zone()
	res.Offset = int64(offset)
	return
}
