package cli

import (
	"fmt"

	"github.com/vshn/k8up/restic/cfg"
	"github.com/vshn/k8up/restic/logging"
)

// Prune will enforce the retention policy onto the repository
func (r *Restic) Prune(tags ArrayOpts) error {
	prunelogger := r.logger.WithName("prune")

	prunelogger.Info("pruning repository")

	args := []string{"--prune"}
	keepN := map[string]*int{
		keepLastArg:    cfg.Config.PruneKeepLast,
		keepHourlyArg:  cfg.Config.PruneKeepHourly,
		keepDailyArg:   cfg.Config.PruneKeepDaily,
		keepWeeklyArg:  cfg.Config.PruneKeepWeekly,
		keepMonthlyArg: cfg.Config.PruneKeepMonthly,
		keepYearlyArg:  cfg.Config.PruneKeepYearly,
	}
	for argName, argVal := range keepN {
		if argVal != nil {
			args = append(args, argName, fmt.Sprintf("%d", *argVal))
		}
	}

	if cfg.Config.PruneKeepTags {
		args = append(args, keepTagsArg)
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
		r.sendPostWebhook()
	}

	return cmd.FatalError
}
