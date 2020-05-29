package restic

import (
	"fmt"
	"os"

	"github.com/vshn/wrestic/kubernetes"
)

// StdinBackup create a snapshot with the data contained in the given reader.
func (r *Restic) StdinBackup(data *kubernetes.ExecData, filename, fileExt string, tags ArrayOpts) error {

	stdinlogger := r.logger.WithName("stdinBackup")

	stdinlogger.Info("starting stdin backup", "filename", filename, "extension", fileExt)

	outputWriter := r.newParseBackupOutput(stdinlogger, filename+fileExt)

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"backup",
			"--stdin",
			"--host",
			os.Getenv(Hostname),
			"--json",
			"--stdin-filename",
			fmt.Sprintf("%s%s", filename, fileExt),
		},
		StdOut: outputWriter,
		StdErr: outputWriter,
		StdIn:  data.Reader,
	}

	return r.triggerBackup(filename+fileExt, stdinlogger, tags, opts, data)
}
