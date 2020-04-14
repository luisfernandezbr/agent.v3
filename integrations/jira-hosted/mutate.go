package main

import (
	"context"

	"github.com/pinpt/agent/rpcdef"
)

func (s *Integration) Mutate(ctx context.Context, fn, data string, config rpcdef.ExportConfig) (res rpcdef.MutateResult, _ error) {
	err := s.initWithConfig(config, false)
	if err != nil {
		return res, err
	}

	return s.common.Mutate(ctx, fn, data, config)
}
