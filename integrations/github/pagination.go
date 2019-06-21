package main

import (
	"time"

	pjson "github.com/pinpt/go-common/json"
)

type checkpoint struct {
	lastProcessed time.Time
	cursors       []string
}

type paginatedFn func(query string, stopOnUpdatedAt time.Time) (pageInfo, error)

const pageSizeStr = "100"

type pageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	EndCursor       string `json:"endCursor"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor"`
}

func paginate(lastProcessed time.Time, fn paginatedFn) error {
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
		q += " orderBy: {field:UPDATED_AT, direction: DESC}"
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

/*
func paginate(fn paginatedFn) error {
	var cursors []string
	for {
		var err error
		cursors, err = fn(cursors)
		if err != nil {
			return err
		}
		if len(cursors) == 0 {
			break
		}
	}
	return nil
}
*/
func makeAfterParam(cursors []string, index int) string {
	if len(cursors) <= index {
		return ""
	}
	v := cursors[index]
	if v == "" {
		return ""
	}
	return "after:" + pjson.Stringify(v)
}
