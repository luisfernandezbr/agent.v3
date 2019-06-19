package main

import (
	pjson "github.com/pinpt/go-common/json"
)

type paginatedFn func(cursors []string) (nextCursors []string, _ error)

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
