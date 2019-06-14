package cmdexport

import (
	"github.com/hashicorp/go-hclog"
)

func Run(logger hclog.Logger) error {
	exp := newExport(logger)
	defer exp.Destroy()
	return nil
}
