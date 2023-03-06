package cli

import (
	"fmt"

	"github.com/k8up-io/k8up/v2/restic/cfg"
	"github.com/k8up-io/k8up/v2/restic/logging"
)

// Prune will enforce the retention policy onto the repository
func (r *Restic) Prune(tags ArrayOpts) error {
	prunelogger := r.logger.WithName("prune")

	prunelogger.Info("pruning repository")

	args := []string{"--prune"}
	keepN := map[string]int{
		"--keep-last":    cfg.Config.PruneKeepLast,
		"--keep-hourly":  cfg.Config.PruneKeepHourly,
		"--keep-daily":   cfg.Config.PruneKeepDaily,
		"--keep-weekly":  cfg.Config.PruneKeepWeekly,
		"--keep-monthly": cfg.Config.PruneKeepMonthly,
		"--keep-yearly":  cfg.Config.PruneKeepYearly,
	}
	for argName, argVal := range keepN {
		if argVal > 0 {
			args = append(args, argName, fmt.Sprintf("%d", argVal))
		}
	}

	keepWithin := map[string]string{
		"--keep-within":         cfg.Config.PruneKeepWithin,
		"--keep-within-hourly":  cfg.Config.PruneKeepWithinHourly,
		"--keep-within-daily":   cfg.Config.PruneKeepWithinDaily,
		"--keep-within-weekly":  cfg.Config.PruneKeepWithinWeekly,
		"--keep-within-monthly": cfg.Config.PruneKeepWithinMonthly,
		"--keep-within-yearly":  cfg.Config.PruneKeepWithinYearly,
	}
	for argName, argVal := range keepWithin {
		if argVal != "" {
			args = append(args, argName, argVal)
		}
	}

	if cfg.Config.PruneKeepTags {
		args = append(args, "--keep-tag")
	}
	if cfg.Config.Hostname != "" {
		args = append(args, "--host="+cfg.Config.Hostname)
	}

	resticPruneLogger := prunelogger.WithName("restic")
	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("forget", args...),
		StdOut: logging.NewInfoWriter(resticPruneLogger),
		StdErr: logging.NewErrorWriter(resticPruneLogger),
	}

	if len(tags) > 0 {
		opts.Args = append(opts.Args, tags.BuildArgs()...)
	}

	cmd := NewCommand(r.ctx, prunelogger, opts)
	cmd.Run()

	if cmd.FatalError == nil {
		r.sendSnapshotList()
	}

	return cmd.FatalError
}
