package cmdexport

func Run(opts Opts) error {
	exp := newExport(opts)
	defer exp.Destroy()
	return nil
}
