package restic

import (
	"bytes"
	"encoding/json"
	"time"
)

// Snapshot models a restic a single snapshot from the
// snapshots --json subcommand.
type Snapshot struct {
	ID       string    `json:"id"`
	Time     time.Time `json:"time"`
	Tree     string    `json:"tree"`
	Paths    []string  `json:"paths"`
	Hostname string    `json:"hostname"`
	Username string    `json:"username"`
	UID      int       `json:"uid"`
	Gid      int       `json:"gid"`
	Tags     []string  `json:"tags"`
}

// Snapshots lists all the snapshots from the repository and saves them in the
// restic instance for further use.
func (r *Restic) Snapshots(tags ArrayOpts) error {
	return r.listSnapshots(tags, false)
}

// LastSnapshots only returns the latests snapshots for a given set of tags.
func (r *Restic) LastSnapshots(tags ArrayOpts) error {
	return r.listSnapshots(tags, true)
}

func (r *Restic) listSnapshots(tags ArrayOpts, last bool) error {
	snaplogger := r.logger.WithName("snapshots")

	snaplogger.Info("getting list of snapshots")

	buf := &bytes.Buffer{}

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"snapshots",
			"--json",
		},
		StdOut: buf,
		StdErr: &outputWrapper{
			parser: &logErrParser{
				log: snaplogger.WithName("restic"),
			},
		},
	}

	if len(tags) > 0 {
		opts.Args = append(opts.Args, tags.BuildArgs()...)
	}

	cmd := NewCommand(r.ctx, snaplogger, opts)
	cmd.Run()

	snaps := []Snapshot{}

	jdecoder := json.NewDecoder(buf)

	err := jdecoder.Decode(&snaps)
	if err != nil {
		return err
	}

	r.snapshots = snaps

	return cmd.FatalError

}
