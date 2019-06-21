package api

import (
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
