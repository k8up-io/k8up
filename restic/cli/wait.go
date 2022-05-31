package cli

import (
	"time"

	"github.com/go-logr/logr"

	"github.com/k8up-io/k8up/v2/restic/logging"
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

// Wait will block as long as there are any locks in the repository. As
// soon as they are all gone the function will return
func (r *Restic) Wait() error {

	waitLogger := r.logger.WithName("WaitForLocks")

	waitLogger.Info("remove old locks")
	err := r.Unlock(false)
	if err != nil {
		return err
	}

	waitLogger.Info("checking for any locks")

	foundLocks := true
	for foundLocks {

		waitLogger.Info("getting a list of active locks")

		locks, err := r.getLockList(waitLogger)
		if err != nil {
			return err
		}

		if len(locks) > 0 {
			foundLocks = true
			waitLogger.Info("locks found, retry in 35 seconds")
			err := r.Unlock(false)
			if err != nil {
				return err
			}
			time.Sleep(35 * time.Second)
		} else {
			foundLocks = false
		}
	}

	waitLogger.Info("no more locks found")

	return nil
}

func (r *Restic) getLockList(log logr.Logger) ([]string, error) {

	list := &locklist{}

	flags := Combine(r.globalFlags, Flags{
		"--json":    {},
		"--no-lock": {},
	})

	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   flags.ApplyToCommand("list", "locks"),
		StdOut: logging.New(list.out),
		StdErr: logging.NewErrorWriter(log.WithName("restic")),
	}
	cmd := NewCommand(r.ctx, log, opts)
	cmd.Run()

	return *list, cmd.FatalError
}

type locklist []string

func (l *locklist) out(s string) {
	*l = append(*l, s)
}
