package restic

import (
	"os"

	"github.com/vshn/wrestic/logging"
)

// Prune will enforce the retention policy onto the repository
func (r *Restic) Prune(tags ArrayOpts) error {
	prunelogger := r.logger.WithName("prune")

	prunelogger.Info("pruning repository")

	args := []string{"--prune"}
	if last := os.Getenv(keepLastEnv); last != "" {
		args = append(args, keepLastArg, last)
	}

	if hourly := os.Getenv(keepHourlyEnv); hourly != "" {
		args = append(args, keepHourlyArg, hourly)
	}

	if daily := os.Getenv(keepDailyEnv); daily != "" {
		args = append(args, keepDailyArg, daily)
	}

	if weekly := os.Getenv(keepWeeklyEnv); weekly != "" {
		args = append(args, keepWeeklyArg, weekly)
	}

	if monthly := os.Getenv(keepMonthlyEnv); monthly != "" {
		args = append(args, keepMonthlyArg, monthly)
	}

	if yearly := os.Getenv(keepYearlyEnv); yearly != "" {
		args = append(args, keepYearlyArg, yearly)
	}

	if keepTags := os.Getenv(keepTagEnv); keepTags != "" {
		args = append(args, keepTagsArg, keepTags)
	}

	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("forget", args...),
		StdOut: logging.NewInfoWriter(prunelogger.WithName("restic")),
		StdErr: logging.NewErrorWriter(prunelogger.WithName("restic")),
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
