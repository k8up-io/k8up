package restic

import "github.com/vshn/wrestic/logging"

// Check will check the repository for errors
func (r *Restic) Check() error {
	checklogger := r.logger.WithName("check")

	checklogger.Info("checking repository")

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"check",
		},
		StdOut: logging.NewInfoWriter(checklogger.WithName("restic")),
		StdErr: logging.NewErrorWriter(checklogger.WithName("restic")),
	}

	cmd := NewCommand(r.ctx, checklogger, opts)
	cmd.Run()

	return cmd.FatalError
}
