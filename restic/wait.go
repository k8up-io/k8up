package restic

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"context"

	"github.com/go-logr/logr"
)

type Lock struct {
	Time      time.Time `json:"time"`
	Exclusive bool      `json:"exclusive"`
	Hostname  string    `json:"hostname"`
	Username  string    `json:"username"`
	Pid       int       `json:"pid"`
	UID       int       `json:"uid"`
	Gid       int       `json:"gid"`
}

// Wait will block as long as there are exclusive locks in the repository. As
// soon as they are all gone the function will return
func (r *Restic) Wait() error {

	waitLogger := r.logger.WithName("WaitForLocks")

	waitLogger.Info("remove old locks")
	err := r.Unlock(false)
	if err != nil {
		return err
	}

	waitLogger.Info("checking for any exclusive locks")

	foundExclusive := true
	for foundExclusive {
		foundExclusive = false

		waitLogger.Info("getting a list of active locks")

		locks, err := r.getLockList(waitLogger)
		if err != nil {
			return err
		}

		for _, lock := range locks {
			if r.isLockExclusive(waitLogger.WithName("lockExclusive"), lock) {
				foundExclusive = true
				waitLogger.Info("found exclusive lock, waiting 10 seconds")
				break
			}
		}
		if foundExclusive {
			time.Sleep(10 * time.Second)
		}
	}

	waitLogger.Info("no more exclusive locks found")

	return nil
}

func (r *Restic) getLockList(log logr.Logger) ([]string, error) {

	list := &locklist{}

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"list",
			"locks",
			"--json",
			"--no-lock",
		},
		StdOut: &outputWrapper{
			parser: list,
		},
		StdErr: &outputWrapper{
			parser: &logErrParser{
				log: log.WithName("restic"),
			},
		},
	}
	cmd := NewCommand(r.ctx, log, opts)
	cmd.Run()

	return []string(*list), cmd.FatalError
}

type locklist []string

func (l *locklist) Parse(s string) error {
	*l = append(*l, s)
	return nil
}

func (r *Restic) isLockExclusive(log logr.Logger, lockID string) bool {

	buf := &bytes.Buffer{}

	wrapper := &outputWrapper{
		parser: &logErrParser{
			log: log.WithName("restic"),
		},
	}

	stdErrBuf := &bytes.Buffer{}

	multiWriter := io.MultiWriter(wrapper, stdErrBuf)

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"cat",
			"lock",
			lockID,
		},
		StdOut: buf,
		StdErr: multiWriter,
	}

	ctx, cancel := context.WithTimeout(r.ctx, time.Second*1)
	defer cancel()

	cmd := NewCommand(ctx, log.WithName("checkLock"), opts)
	cmd.Run()

	// if there was an error getting the locks we assume exclusive lock.
	// But ignore errors where the lock isn't found anymore, in that case we return false
	if strings.Contains(stdErrBuf.String(), "The specified key does not exist") {
		log.Info("ignoring missing lock")
		return false
	} else if cmd.FatalError != nil {
		return true
	}

	lock := &Lock{}

	err := json.Unmarshal(buf.Bytes(), lock)

	if err != nil {
		return false
	}

	return lock.Exclusive
}
