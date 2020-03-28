package restic

import (
	"fmt"
	"io"
	"os"
)

// StdinBackup create a snapshot with the data contained in the given reader.
func (r *Restic) StdinBackup(data io.ReadCloser, filename, fileExt string, tags ArrayOpts) error {

	stdinlogger := r.logger.WithName("stdinBackup")

	stdinlogger.Info("starting stdin backup", "filename", filename, "extension", fileExt)

	readPipe, writePipe := io.Pipe()
	defer readPipe.Close()
	defer writePipe.Close()

	go r.parseBackupOutput(readPipe, stdinlogger, filename+fileExt)

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
		StdOut: writePipe,
		StdErr: writePipe,
		StdIn:  data,
	}

	if len(tags) > 0 {
		opts.Args = append(opts.Args, tags.BuildArgs()...)
	}

	cmd := NewCommand(r.ctx, stdinlogger, opts)
	cmd.Run()

	data.Close()

	return cmd.FatalError
}
