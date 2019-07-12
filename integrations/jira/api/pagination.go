package api

import (
	"errors"
	"net/url"
	"strconv"
)

type PaginateStartAtFn func(paginationParams url.Values) (hasMore bool, pageSize int, _ error)

func PaginateStartAt(fn PaginateStartAtFn) error {
	pageOffset := 0
	for {
		q := url.Values{}
		q.Add("startAt", strconv.Itoa(pageOffset))
		hasMore, pageSize, err := fn(q)
		if err != nil {
			return err
		}
		if pageSize == 0 {
			return errors.New("pageSize is 0")
		}
		if !hasMore {
			return nil
		}
		pageOffset += pageSize
	}
}
