package cmdservicestart

import (
	"context"

	"github.com/pinpt/agent/cmd/cmdrun"
	"github.com/pinpt/agent/pkg/service"
)

func Run(ctx context.Context, opts cmdrun.Opts) error {
	return service.Run(service.Start, ctx, opts)
}
