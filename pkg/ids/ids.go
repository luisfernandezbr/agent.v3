package ids

import "github.com/pinpt/go-common/hash"

func basicID(kind string, customerID string, refType string, refID string) string {
	return hash.Values(kind, customerID, refType, refID)
}

func CodeRepo(customerID string, refType string, refID string) string {
	return basicID("Repo", customerID, refType, refID)
}

func CodeUser(customerID string, refType string, refID string) string {
	return basicID("User", customerID, refType, refID)
}

func CodePullRequest(customerID string, refType string, refID string) string {
	return basicID("PullRequest", customerID, refType, refID)
}

func CodeCommit(customerID string, refType string, commitSHA string) string {
	return hash.Values("Commit", customerID, refType, commitSHA)
}

func CodeCommits(customerID string, refType string, commitSHAs []string) (res []string) {
	for _, sha := range commitSHAs {
		res = append(res, CodeCommit(customerID, refType, sha))
	}
	return
}

func CodeBranch(customerID string, refType string, repoRefID string, branchName string) string {
	repoID := CodeRepo(customerID, refType, repoRefID)
	return hash.Values(refType, repoID, customerID, branchName)
}

func WorkProject(customerID string, refType string, refID string) string {
	return basicID("Project", customerID, refType, refID)
}

func WorkIssue(customerID string, refType string, refID string) string {
	return basicID("Issue", customerID, refType, refID)
}

func WorkUser(customerID string, refType string, refID string) string {
	return basicID("User", customerID, refType, refID)
}
