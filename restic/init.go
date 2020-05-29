package restic

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// Init initialises a repository, checks if the repositor exists and will
// initialise it if not. It's save to call this every time.
func (r *Restic) Init() error {

	initLogger := r.logger.WithName("RepoInit")
	resticLogger := initLogger.WithName("restic")

	initErrorCatcher := &initStdErrWrapper{
		exists: false,
		Writer: &outputWrapper{
			parser: &logErrParser{
				log: resticLogger,
			},
		},
	}

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"init",
		},
		StdOut: &outputWrapper{
			parser: &logOutParser{
				log: resticLogger,
			},
		},
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
