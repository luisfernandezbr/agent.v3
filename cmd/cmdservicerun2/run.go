package cmdservicerun2

import (
	"github.com/hashicorp/go-hclog"
)

type Opts struct {
	Logger       hclog.Logger
	PinpointRoot string
}

func Run(opts Opts) error {
	run, err := newRunner(opts)
	if err != nil {
		return err
	}
	return run.run()
}

type runner struct {
	opts   Opts
	logger hclog.Logger
}

func newRunner(opts Opts) (*runner, error) {
	s := &runner{}
	s.opts = opts
	s.logger = opts.Logger
	return s, nil
}

func (s *runner) run() error {

	s.logger.Info("starting service")
	return nil
}
