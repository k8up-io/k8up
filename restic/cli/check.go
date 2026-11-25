package cli

import (
	"github.com/k8up-io/k8up/v2/restic/logging"
)

// Check will check the repository for errors.
// If readDataSubset is not empty, it is passed to "restic check --read-data-subset"
// so that part of the pack data gets read and verified as well.
func (r *Restic) Check(readDataSubset string) error {
	checkLogger := r.logger.WithName("check")

	checkLogger.Info("checking repository")

	var args []string
	if readDataSubset != "" {
		args = append(args, "--read-data-subset="+readDataSubset)
	}

	resticCheckLogger := checkLogger.WithName("restic")
	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("check", args...),
		StdOut: logging.NewInfoWriter(resticCheckLogger),
		StdErr: logging.NewErrorWriter(resticCheckLogger),
	}

	cmd := NewCommand(r.ctx, checkLogger, opts)
	cmd.Run()

	return cmd.FatalError
}
