package api

type CommitAuthor struct {
	CommitHash  string
	AuthorName  string
	AuthorEmail string
	// AuthorRefID    string
	CommitterName  string
	CommitterEmail string
	// CommitterRefID string
}
