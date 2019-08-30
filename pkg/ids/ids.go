package ids

import "github.com/pinpt/go-common/hash"

func SourcecodeCommitID(customerID string, refType string, commitSHA string) string {
	return hash.Values("Commit", customerID, refType, commitSHA)
}
