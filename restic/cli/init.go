package cli

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"github.com/k8up-io/k8up/v2/restic/logging"
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

	// array of acceptable errors to attempt to continue
	okErrorArray := []string{"already initialized", "already exists"}

	for scanner.Scan() {
		for _, errorString := range okErrorArray {
			if strings.Contains(scanner.Text(), errorString) {
				i.exists = true
				return len(p), nil
			}
		}
	}

	return i.Writer.Write(p)
}
