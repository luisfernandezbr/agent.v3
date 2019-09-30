package api

import "time"

// used in reposResponseLight struct and projectResponse struct
type projectResponseLight struct {
	ID             string `json:"id"`
	LastUpdateTime string `json:"lastUpdateTime"` // not in TFS
	Name           string `json:"name"`
	State          string `json:"state"`
}

// used in reposResponse struct
type projectResponse struct {
	projectResponseLight
	Revision    int64  `json:"revision"`
	State       string `json:"state"`
	URL         string `json:"url"`
	Visibility  string `json:"visibility"`
	Description string `json:"description"`
}

// used in reposResponse struct
type reposResponseLight struct {
	ID      string               `json:"id"`
	Name    string               `json:"name"`
	Project projectResponseLight `json:"project"`
	URL     string               `json:"url"`
}

// used in src_repos.go - fetchRepos
type reposResponse struct {
	reposResponseLight
	DefaultBranch string          `json:"defaultBranch"`
	Project       projectResponse `json:"project"`
	RemoteURL     string          `json:"remoteUrl"`
	Size          int64           `json:"size"`   // not in TFS
	SSHURL        string          `json:"sshUrl"` // not in TFS
	WebURL        string          `json:"webUrl"` // not in TFS
}

// used in src_pull_requests.go - fetchPullRequests
type pullRequestResponse struct {
	ClosedDate          time.Time     `json:"closedDate"`
	CodeReviewID        int64         `json:"codeReviewId"`
	CreatedBy           usersResponse `json:"createdBy"`
	CreationDate        time.Time     `json:"creationDate"`
	CompletionQueueTime time.Time     `json:"completionQueueTime"`
	Description         string        `json:"description"`
	IsDraft             bool          `json:"isDraft"`
	LastMergeCommit     struct {
		CommidID string `json:"commitId"`
		URL      string `json:"url"`
	} `json:"lastMergeCommit"`
	LastMergeSourceCommit struct {
		CommidID string `json:"commitId"`
		URL      string `json:"url"`
	} `json:"lastMergeSourceCommit"`
	LastMergeTargetCommit struct {
		CommidID string `json:"commitId"`
		URL      string `json:"url"`
	} `json:"lastMergeTargetCommit"`
	MergeID       string             `json:"mergeId"`
	MergeStatus   string             `json:"mergeStatus"`
	PullRequestID int64              `json:"pullRequestId"`
	Repository    reposResponseLight `json:"repository"`
	Reviewers     []struct {
		DisplayName string `json:"displayName"`
		ID          string `json:"id"`
		ImageURL    string `json:"imageUrl"`
		IsFlagged   bool   `json:"isFlagged"`
		ReviewerURL string `json:"reviewerUrl"`
		UniqueName  string `json:"uniqueName"`
		URL         string `json:"url"`
		Vote        int64  `json:"vote"`
	} `json:"reviewers"`
	SourceBranch       string `json:"sourceRefName"`
	Status             string `json:"status"`
	SupportsIterations bool   `json:"supportsIterations"`
	TargetBranch       string `json:"targetRefName"`
	Title              string `json:"title"`
	URL                string `json:"url"`
}

type pullRequestResponseWithShas struct {
	pullRequestResponse
	commitshas []string
}

// used in src_pull_request_commits.go - fetchPullRequestCommits
type commitsResponseLight struct {
	Author struct {
		Date  time.Time `json:"date"`
		Email string    `json:"email"`
		Name  string    `json:"name"`
	} `json:"author"`
	Comment   string `json:"comment"`
	CommitID  string `json:"commitId"`
	Committer struct {
		Date  time.Time `json:"date"`
		Email string    `json:"email"`
		Name  string    `json:"name"`
	} `json:"committer"`
	URL          string `json:"url"`
	ChangeCounts struct {
		Add    int `json:"Add"`
		Delete int `json:"Delete"`
		Edit   int `json:"Edit"`
	} `json:"changeCounts"`
}

// used in src_commit_users.go - fetchCommits
type commitsResponse struct {
	commitsResponseLight
	RemoteURL    string `json:"remoteUrl"`
	ChangeCounts struct {
		Add    int64 `json:"Add"`
		Delete int64 `json:"Delete"`
		Edit   int64 `json:"Edit"`
	} `json:"changeCounts"`
}

// CommitResponse used in src_commit_users.go - FetchLastCommit
type CommitResponse struct {
	Author struct {
		Date  time.Time `json:"date"`
		Email string    `json:"email"`
		Name  string    `json:"name"`
	} `json:"author"`
	Comment   string `json:"comment"`
	CommitID  string `json:"commitId"`
	Committer struct {
		Date  time.Time `json:"date"`
		Email string    `json:"email"`
		Name  string    `json:"name"`
	} `json:"committer"`
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
}

// used in src_pull_request_comments.go - fetchPullRequestComments
type commentsReponse struct {
	Comments []struct {
		Author                 usersResponse `json:"author"`
		CommentType            string        `json:"commentType"`
		Content                string        `json:"content"`
		ID                     int64         `json:"id"`
		LastContentUpdatedDate time.Time     `json:"lastContentUpdatedDate"`
		LastUpdatedDate        time.Time     `json:"lastUpdatedDate"`
		ParentCommentID        int64         `json:"parentCommentId"`
		PublishedDate          time.Time     `json:"publishedDate"`
	} `json:"comments"`
	ID              int64                    `json:"id"`
	Identities      map[string]usersResponse `json:"identities"`
	IsDeleted       bool                     `json:"isDeleted"`
	LastUpdatedDate time.Time                `json:"lastUpdatedDate"`
	PublishedDate   time.Time                `json:"publishedDate"`
}

// used in scr_pull_requests.go - fetchSingleCommit
type singleCommitResponse struct {
	Author struct {
		Date     time.Time `json:"date"`
		Email    string    `json:"email"`
		ImageURL string    `json:"imageUrl"`
		Name     string    `json:"name"`
	} `json:"author"`
	ChangeCounts struct {
		Add    int64 `json:"Add"`
		Delete int64 `json:"Delete"`
		Edit   int64 `json:"Edit"`
	} `json:"changeCounts"`
	Comment   string `json:"comment"`
	Committer struct {
		Date     time.Time `json:"date"`
		Email    string    `json:"email"`
		ImageURL string    `json:"imageUrl"`
		Name     string    `json:"name"`
	} `json:"committer"`
	Push struct {
		Date     time.Time     `json:"date"`
		PushedBy usersResponse `json:"pushedBy"`
	} `json:"push"`
	RemoteURL string `json:"remoteUrl"`
}
