package cli

import (
	"github.com/k8up-io/k8up/v2/restic/logging"
)

// Check will check the repository for errors
func (r *Restic) Check() error {
	checkLogger := r.logger.WithName("check")

	checkLogger.Info("checking repository")

	resticCheckLogger := checkLogger.WithName("restic")
	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("check"),
		StdOut: logging.NewInfoWriter(resticCheckLogger),
		StdErr: logging.NewErrorWriter(resticCheckLogger),
	}

	cmd := NewCommand(r.ctx, checkLogger, opts)
	cmd.Run()

	return cmd.FatalError
}
