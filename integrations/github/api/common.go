package api

import (
	"github.com/hashicorp/go-hclog"
)

type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	EndCursor       string `json:"endCursor"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor"`
}

type IDs []string

type QueryContext struct {
	Logger  hclog.Logger
	Request func(query string, res interface{}) error

	APIURL3 string

	CustomerID    string
	RepoID        func(ref string) string
	UserID        func(ref string) string
	PullRequestID func(ref string) string
	BranchID      func(repoRef, branchName string) string

	UserLoginToRefID           func(login string) (refID string, _ error)
	UserLoginToRefIDFromCommit func(login, email string) (refID string, _ error)

	IsEnterprise func() bool
}

func (s QueryContext) WithLogger(logger hclog.Logger) QueryContext {
	res := s
	res.Logger = logger
	return res
}
