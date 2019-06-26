package api

import (
	"time"

	"github.com/hashicorp/go-hclog"
	pjson "github.com/pinpt/go-common/json"
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

	CustomerID    string
	RepoID        func(ref string) string
	UserID        func(ref string) string
	PullRequestID func(ref string) string

	UserLoginToRefID func(login string) (refID string, _ error)
}

type PaginateRegularFn func(query string) (PageInfo, error)

const pageSizeStr = "100"

func PaginateRegular(fn PaginateRegularFn) error {
	cursor := ""
	for {
		q := "first: " + pageSizeStr
		if cursor != "" {
			q += " after:" + pjson.Stringify(cursor)
		}
		pi, err := fn(q)
		if err != nil {
			return err
		}
		if pi.HasNextPage {
			cursor = pi.EndCursor
		} else {
			return nil
		}
	}
}

type PaginateNewerThanFn func(query string, stopOnUpdatedAt time.Time) (PageInfo, error)

// PaginateNewerThan is pagination for resources supporting orderBy UPDATED_AT field.
func PaginateNewerThan(lastProcessed time.Time, fn PaginateNewerThanFn) error {
	if lastProcessed.IsZero() {
		cursor := ""
		for {
			q := "first: " + pageSizeStr
			if cursor != "" {
				q += " after:" + pjson.Stringify(cursor)
			}
			q += " orderBy: {field:UPDATED_AT, direction: ASC}"
			pi, err := fn(q, time.Time{})
			if err != nil {
				return err
			}
			if pi.HasNextPage {
				cursor = pi.EndCursor
			} else {
				return nil
			}
		}
	}
	cursor := ""
	for {
		q := "last: " + pageSizeStr
		if cursor != "" {
			q += " before:" + pjson.Stringify(cursor)
		}
		q += " orderBy: {field:UPDATED_AT, direction: ASC}"
		pi, err := fn(q, lastProcessed)
		if err != nil {
			return err
		}
		if pi.HasPreviousPage {
			cursor = pi.StartCursor
		} else {
			return nil
		}
	}
}

type PaginateCommitsFn func(query string) (PageInfo, error)

const iso8601format = "2006-01-02T15:04:05-0700"

// PaginateCommits is pagination for commit history which supports since argument.
func PaginateCommits(lastProcessed time.Time, fn PaginateCommitsFn) error {
	cursor := ""
	since := ""
	if !lastProcessed.IsZero() {
		since = " since: " + pjson.Stringify(lastProcessed.Format(iso8601format))
	}

	for {
		q := "first: " + pageSizeStr + since
		if cursor != "" {
			q += " after:" + pjson.Stringify(cursor)
		}
		pi, err := fn(q)
		if err != nil {
			return err
		}
		if pi.HasNextPage {
			cursor = pi.EndCursor
		} else {
			return nil
		}
	}
}

func strInArr(str string, arr []string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}
