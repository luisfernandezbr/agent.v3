package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	pjson "github.com/pinpt/go-common/v10/json"
)

type PaginateRegularFn func(query string) (PageInfo, error)

const defaultPageSize = 100

func PaginateRegular(fn PaginateRegularFn) error {
	return PaginateRegularWithPageSize(defaultPageSize, fn)
}

func PaginateRegularWithPageSize(pageSize int, fn PaginateRegularFn) error {
	pageSizeStr := strconv.Itoa(pageSize)

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

func PaginateNewerThan(lastProcessed time.Time, fn PaginateNewerThanFn) error {
	return PaginateNewerThanWithPageSize(lastProcessed, defaultPageSize, fn)
}

// PaginateNewerThan is pagination for resources supporting orderBy UPDATED_AT field.
func PaginateNewerThanWithPageSize(lastProcessed time.Time, pageSize int, fn PaginateNewerThanFn) error {
	if lastProcessed.IsZero() {
		cursor := ""
		for {
			q := "first: " + strconv.Itoa(pageSize)
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
		q := "first: " + strconv.Itoa(pageSize)
		if cursor != "" {
			q += " after:" + pjson.Stringify(cursor)
		}
		q += " orderBy: {field:UPDATED_AT, direction: DESC}"
		pi, err := fn(q, lastProcessed)
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

type PaginateCommitsFn func(query string) (PageInfo, error)

// PaginateCommits is pagination for commit history which supports since argument.
func PaginateCommits(lastProcessed time.Time, fn PaginateCommitsFn) error {
	cursor := ""
	since := ""
	if !lastProcessed.IsZero() {
		// Since parameter format used in GitHub.com (2019-08-21) can include time zone (2006-01-02T15:04:05-0700)
		// But GitHub Enterprise 2.15.9 requires the date to be in UTC.
		iso8601z := "2006-01-02T15:04:05Z"
		since = " since: " + pjson.Stringify(lastProcessed.UTC().Format(iso8601z))
	}

	for {
		q := "first: " + strconv.Itoa(defaultPageSize) + since
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

type PaginateV3Fn func(u string) (responseHeaders http.Header, _ error)

func PaginateV3(fn PaginateV3Fn) error {
	u := ""
	i := 0
	for {
		i++
		if i > 10000 {
			panic("more than 10000 pages found. since we only paginate orgs, this is likely a bug")
		}
		responseHeaders, err := fn(u)
		if err != nil {
			return err
		}
		u, err = getNextFromLinkHeader(responseHeaders.Get("Link"))
		if err != nil {
			return err
		}
		if u == "" {
			break
		}
	}
	return nil
}

func getNextFromLinkHeader(link string) (string, error) {
	links := strings.Split(link, ",")
	for _, link := range links {
		link = strings.TrimSpace(link)
		parts := strings.Split(link, ";")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid link header %v", link)
		}
		wantedRel := `rel="next"`
		if strings.TrimSpace(parts[1]) == wantedRel {
			u := strings.Trim(parts[0], "<>")
			return u, nil
		}
	}
	return "", nil
}
