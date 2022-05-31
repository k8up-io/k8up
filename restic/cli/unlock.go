package cli

import (
	"github.com/k8up-io/k8up/v2/restic/logging"
)

// Unlock will remove stale locks from the repository
// If the all flag is set to true, even non-stale locks are removed.
func (r *Restic) Unlock(all bool) error {
	unlocklogger := r.logger.WithName("unlock")

	unlocklogger.Info("unlocking repository", "all", all)

	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("unlock"),
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
