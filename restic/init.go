package restic

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"github.com/vshn/wrestic/logging"
)

// Init initialises a repository, checks if the repositor exists and will
// initialise it if not. It's save to call this every time.
func (r *Restic) Init() error {

	initLogger := r.logger.WithName("RepoInit")
	resticLogger := initLogger.WithName("restic")

	initErrorCatcher := &initStdErrWrapper{
		exists: false,
		Writer: logging.NewErrorWriter(resticLogger),
	}

	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("init"),
		StdOut: logging.NewInfoWriter(resticLogger),
		StdErr: initErrorCatcher,
	}
	cmd := NewCommand(r.ctx, initLogger, opts)
	cmd.Run()

	if !initErrorCatcher.exists {
		return cmd.FatalError
	}

	return nil
}

type initStdErrWrapper struct {
	exists bool
	io.Writer
}

func (i *initStdErrWrapper) Write(p []byte) (n int, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(p))

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "already initialized") {
			i.exists = true
			return len(p), nil
		}
	}
	return i.Writer.Write(p)
}
