package restic

import (
	"fmt"
	"os"

	"github.com/vshn/wrestic/kubernetes"
	"github.com/vshn/wrestic/logging"
)

// StdinBackup create a snapshot with the data contained in the given reader.
func (r *Restic) StdinBackup(data *kubernetes.ExecData, filename, fileExt string, tags ArrayOpts) error {

	stdinlogger := r.logger.WithName("stdinBackup")

	stdinlogger.Info("starting stdin backup", "filename", filename, "extension", fileExt)

	outputWriter := logging.NewStdinBackupOutputParser(stdinlogger.WithName("progress"), filename+fileExt, r.sendBackupStats)

	flags := Combine(r.globalFlags, Flags{
		"--host":           {os.Getenv(Hostname)},
		"--json":           {},
		"--stdin":          {},
		"--stdin-filename": {fmt.Sprintf("%s%s", filename, fileExt)},
	})

	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   flags.ApplyToCommand("backup"),
		StdOut: outputWriter,
		StdErr: outputWriter,
		StdIn:  data.Reader,
	}

	return r.triggerBackup(stdinlogger, tags, opts, data)
}
