package api

import (
	"net/url"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/ids"
)

type QueryContext struct {
	BaseURL        string
	Logger         hclog.Logger
	Request        func(objPath string, params url.Values, res interface{}) (PageInfo, error)
	RequestGraphQL func(query string, res interface{}) (err error)

	CustomerID string
	RefType    string

	UserEmailMap map[string]string
}

type PageInfo struct {
	PageSize   int
	NextPage   string
	Page       string
	TotalPages string
}

func (s QueryContext) RepoID(refID string) string {
	return ids.CodeRepo(s.CustomerID, s.RefType, refID)
}

func (s QueryContext) BranchID(repoID, branchName, firstCommitSHA string) string {
	return ids.CodeBranch(s.CustomerID, s.RefType, repoID, branchName, firstCommitSHA)
}

func (s QueryContext) PullRequestID(repoID, refID string) string {
	return ids.CodePullRequest(s.CustomerID, s.RefType, repoID, refID)
}
