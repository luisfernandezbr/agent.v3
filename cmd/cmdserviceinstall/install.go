package cmdserviceinstall

import (
	"context"

	"github.com/pinpt/agent/cmd/cmdrun"
	"github.com/pinpt/agent/pkg/service"
)

func Run(ctx context.Context, opts cmdrun.Opts, start bool) error {

	if err := service.Run(service.Install, nil, opts); err != nil {
		return err
	}

	if start {
		return service.Run(service.Start, ctx, opts)
	}

	return nil
}
