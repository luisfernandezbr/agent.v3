package ids2

import (
	"github.com/pinpt/integration-sdk/sourcecode"
	"github.com/pinpt/integration-sdk/work"
)

type Gen struct {
	customerID string
	refType    string
}

func New(customerID, refType string) Gen {
	return Gen{
		customerID: customerID,
		refType:    refType,
	}
}

func (s Gen) CodeRepo(refID string) string {
	return sourcecode.NewRepoID(s.customerID, s.refType, refID)
}

func (s Gen) CodeUser(refID string) string {
	return sourcecode.NewUserID(s.customerID, s.refType, refID)
}

func (s Gen) CodePullRequest(repoID string, refID string) string {
	return sourcecode.NewPullRequestID(s.customerID, refID, s.refType, repoID)
}

func (s Gen) CodeCommit(repoID string, commitSHA string) string {
	return sourcecode.NewCommitID(s.customerID, commitSHA, s.refType, repoID)
}

func (s Gen) CodeCommits(repoID string, commitSHAs []string) (res []string) {
	for _, sha := range commitSHAs {
		res = append(res, s.CodeCommit(repoID, sha))
	}
	return
}

func (s Gen) CodeBranch(repoID string, branchName string, firstCommitSHA string) string {
	firstCommitID := s.CodeCommit(repoID, firstCommitSHA)
	return sourcecode.NewBranchID(s.refType, repoID, s.customerID, branchName, firstCommitID)
}

func (s Gen) WorkProject(refID string) string {
	return work.NewProjectID(s.customerID, s.refType, refID)
}

func (s Gen) WorkIssue(refID string) string {
	return work.NewIssueID(s.customerID, s.refType, refID)
}

func (s Gen) WorkUser(refID string) string {
	return work.NewUserID(s.customerID, s.refType, refID)
}
