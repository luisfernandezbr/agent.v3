package ids

import (
	"reflect"

	"github.com/pinpt/integration-sdk/sourcecode"
	"github.com/pinpt/integration-sdk/work"
)

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

type BasicInfo struct {
	CustomerID string
	RefType    string
}

func getBasicInfo(conf interface{}) BasicInfo {

	t := reflect.ValueOf(conf)

	return BasicInfo{
		CustomerID: t.FieldByName("CustomerID").Interface().(string),
		RefType:    t.FieldByName("RefType").Interface().(string),
	}
}

func RepoID(refID string, info interface{}) string {
	s := getBasicInfo(info)
	return CodeRepo(s.CustomerID, s.RefType, refID)
}

func BranchID(repoID, branchName, firstCommitSHA string, info interface{}) string {
	s := getBasicInfo(info)
	return CodeBranch(s.CustomerID, s.RefType, repoID, branchName, firstCommitSHA)
}

func PullRequestID(repoID, refID string, info interface{}) string {
	s := getBasicInfo(info)
	return CodePullRequest(s.CustomerID, s.RefType, repoID, refID)
}
