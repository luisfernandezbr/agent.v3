package api

type PaginateFn func(nextPage NextPage) (NextPage, error)

func Paginate(fn PaginateFn) (rerr error) {

	var nextPage NextPage
	for {
		nextPage, rerr = fn(nextPage)
		if rerr != nil || nextPage == "" {
			return
		}
	}
}
