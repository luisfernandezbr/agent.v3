package api

import (
	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/agent/pkg/reqstats"
)

type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	EndCursor       string `json:"endCursor"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor"`
}

type IDs []string

type QueryContext struct {
	Logger hclog.Logger

	Request func(query string, vars map[string]interface{}, res interface{}) error

	APIURL  string
	APIURL3 string

	CustomerID string
	RefType    string

	// deprecated, use ExportUserUsingFullDetails instead
	UserLoginToRefID           func(login string) (refID string, _ error)
	UserLoginToRefIDFromCommit func(logger hclog.Logger, login, name, email string) (refID string, _ error)
	ExportUserUsingFullDetails func(logger hclog.Logger, user User) (refID string, _ error)

	IsEnterprise func() bool

	Clients reqstats.Clients

	AuthToken string
}

func (s QueryContext) WithLogger(logger hclog.Logger) QueryContext {
	res := s
	res.Logger = logger
	return res
}

func (s QueryContext) RepoID(refID string) string {
	return ids.CodeRepo(s.CustomerID, s.RefType, refID)
}

func (s QueryContext) UserID(refID string) string {
	return ids.CodeUser(s.CustomerID, s.RefType, refID)
}

func (s QueryContext) PullRequestID(repoID, refID string) string {
	return ids.CodePullRequest(s.CustomerID, s.RefType, repoID, refID)
}

func (s QueryContext) BranchID(repoID, branchName, firstCommitSHA string) string {
	return ids.CodeBranch(s.CustomerID, s.RefType, repoID, branchName, firstCommitSHA)
}
