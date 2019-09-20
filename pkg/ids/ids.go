package ids

import (
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
	"github.com/pinpt/integration-sdk/work"
)

func CodeCommitEmail(customerID string, email string) string {
	return hash.Values(customerID, email)
}

func CodeRepo(customerID string, refType string, refID string) string {
	return sourcecode.NewRepoID(customerID, refType, refID)
}

func CodeUser(customerID string, refType string, refID string) string {
	return sourcecode.NewUserID(customerID, refType, refID)
}

func CodePullRequest(customerID string, refType string, repoID string, refID string) string {
	return sourcecode.NewPullRequestID(customerID, refID, refType, repoID)
}

func CodeCommit(customerID string, refType string, repoID string, commitSHA string) string {
	return sourcecode.NewCommitID(customerID, commitSHA, refType, repoID)
}

func CodeCommits(customerID string, refType string, repoID string, commitSHAs []string) (res []string) {
	for _, sha := range commitSHAs {
		res = append(res, CodeCommit(customerID, refType, repoID, sha))
	}
	return
}

func CodeBranch(customerID string, refType string, repoID string, branchName string, firstCommitSHA string) string {
	firstCommitID := CodeCommit(customerID, refType, repoID, firstCommitSHA)
	return sourcecode.NewBranchID(refType, repoID, customerID, branchName, firstCommitID)
}

func WorkProject(customerID string, refType string, refID string) string {
	return work.NewProjectID(customerID, refType, refID)
}

func WorkIssue(customerID string, refType string, refID string) string {
	return work.NewIssueID(customerID, refType, refID)
}

func WorkUser(customerID string, refType string, refID string) string {
	return work.NewUserID(customerID, refType, refID)
}
