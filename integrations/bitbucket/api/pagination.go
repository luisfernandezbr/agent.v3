package api

import (
	"github.com/hashicorp/go-hclog"
)

type PaginateFn func(log hclog.Logger, nextPage NextPage) (NextPage, error)

func Paginate(log hclog.Logger, fn PaginateFn) (rerr error) {

	var nextPage NextPage
	for {
		nextPage, rerr = fn(log, nextPage)
		if rerr != nil {
			return
		}
		if nextPage == "" {
			return nil
		}
	}
}
