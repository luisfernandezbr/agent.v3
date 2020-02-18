package main

import (
	"context"
	"errors"

	"github.com/pinpt/agent/rpcdef"
)

func (s *Integration) Mutate(ctx context.Context, fn, data string, config rpcdef.ExportConfig) (res rpcdef.MutatedObjects, rerr error) {
	rerr = errors.New("mutate not supported")
	return
}
