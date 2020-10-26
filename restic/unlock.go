package restic

import "github.com/vshn/wrestic/logging"

// Unlock will remove stale locks from the repository
func (r *Restic) Unlock(all bool) error {
	unlocklogger := r.logger.WithName("unlock")

	unlocklogger.Info("unlocking repository", "all", all)

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"unlock",
		},
		StdOut: logging.NewErrorWriter(unlocklogger.WithName("restic")),
		StdErr: logging.NewErrorWriter(unlocklogger.WithName("restic")),
	}

	if all {
		opts.Args = append(opts.Args, "--remove-all")
	}

	cmd := NewCommand(r.ctx, unlocklogger, opts)
	cmd.Run()

	return cmd.FatalError
}
