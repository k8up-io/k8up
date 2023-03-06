package cli

import (
	"bytes"
	"encoding/json"

	"github.com/k8up-io/k8up/v2/restic/dto"
	"github.com/k8up-io/k8up/v2/restic/logging"
)

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
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("snapshots", "--json"),
		StdOut: buf,
		StdErr: logging.NewErrorWriter(snaplogger.WithName("restic")),
	}

	if len(tags) > 0 {
		opts.Args = append(opts.Args, tags.BuildArgs()...)
	}

	cmd := NewCommand(r.ctx, snaplogger, opts)
	cmd.Run()

	snaps := []dto.Snapshot{}

	jdecoder := json.NewDecoder(buf)

	err := jdecoder.Decode(&snaps)
	if err != nil {
		return err
	}

	r.snapshots = snaps

	return cmd.FatalError

}
