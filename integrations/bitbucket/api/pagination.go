package api

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/go-hclog"
)

type PaginateStartAtFn func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error)

func Paginate(log hclog.Logger, fn PaginateStartAtFn) error {
	nextPage := "1"
	for {
		q := url.Values{}
		q.Set("page", nextPage)
		q.Set("pagelen", "100")
		pageInfo, err := fn(log, q)
		if err != nil {
			return err
		}
		log.Debug("page", "info", fmt.Sprintf("%+v", pageInfo))
		if pageInfo.NextPage == "" {
			return nil
		}
		if pageInfo.PageSize == 0 {
			return errors.New("pageSize is 0")
		}

		nextPage = pageInfo.NextPage
	}
}

type PaginateNewerThanFn func(log hclog.Logger, parameters url.Values, stopOnUpdatedAt time.Time) (PageInfo, error)

func PaginateNewerThan(log hclog.Logger, lastProcessed time.Time, fn PaginateNewerThanFn) error {
	nextPage := "1"
	for {
		p := url.Values{}
		p.Set("page", nextPage)
		pageInfo, err := fn(log, p, lastProcessed)
		if err != nil {
			return err
		}
		log.Debug("page", "info", fmt.Sprintf("%+v", pageInfo))
		if pageInfo.NextPage == "" {
			return nil
		}
		if pageInfo.PageSize == 0 {
			return errors.New("pageSize is 0")
		}
		nextPage = pageInfo.NextPage
	}
}
