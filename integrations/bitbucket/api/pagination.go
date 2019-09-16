package api

import (
	"errors"
	"net/url"

	"github.com/hashicorp/go-hclog"
)

type PaginateStartAtFn func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error)

func Paginate(log hclog.Logger, fn PaginateStartAtFn) error {
	nextPage := "1"
	for {
		q := url.Values{}
		q.Add("page", nextPage)
		pageInfo, err := fn(log, q)
		if err != nil {
			return err
		}
		if pageInfo.NextPage == "" {
			return nil
		}
		if pageInfo.PageSize == 0 {
			return errors.New("pageSize is 0")
		}

		nextPage = pageInfo.NextPage
	}
}
