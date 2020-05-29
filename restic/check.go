package restic

// Check will check the repository for errors
func (r *Restic) Check() error {
	checklogger := r.logger.WithName("check")

	checklogger.Info("checking repository")

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"check",
		},
		StdOut: &outputWrapper{
			parser: &logOutParser{
				log: checklogger.WithName("restic"),
			},
		},
		StdErr: &outputWrapper{
			parser: &logErrParser{
				log: checklogger.WithName("restic"),
			},
		},
	}

	cmd := NewCommand(r.ctx, checklogger, opts)
	cmd.Run()

	return cmd.FatalError
}
