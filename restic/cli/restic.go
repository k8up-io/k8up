package cli

import (
	"context"
	"path"
	"strings"

	"github.com/go-logr/logr"

	"github.com/vshn/k8up/restic/cfg"
)

const (
	keepLastArg    = "--keep-last"
	keepHourlyArg  = "--keep-hourly"
	keepDailyArg   = "--keep-daily"
	keepWeeklyArg  = "--keep-weekly"
	keepMonthlyArg = "--keep-monthly"
	keepYearlyArg  = "--keep-yearly"
	keepTagsArg    = "--keep-tag"
)

type ArrayOpts []string

func (a *ArrayOpts) String() string {
	return strings.Join(*a, ", ")
}

func (a *ArrayOpts) BuildArgs() []string {
	argList := make([]string, 0)
	for _, elem := range *a {
		argList = append(argList, "--tag", elem)
	}
	return argList
}

func (a *ArrayOpts) Set(value string) error {
	*a = append(*a, value)
	return nil
}

type Restic struct {
	resticPath string
	logger     logr.Logger
	snapshots  []Snapshot
	ctx        context.Context
	bucket     string

	// globalFlags are applied to all invocations of restic
	globalFlags  Flags
	statsHandler StatsHandler
}

// New returns a new Restic reference
func New(ctx context.Context, logger logr.Logger, statsHandler StatsHandler) *Restic {
	globalFlags := Flags{}

	options := strings.Split(cfg.Config.ResticOptions, ",")
	if len(options) > 0 {
		logger.Info("using the following restic options", "options", options)
		globalFlags.AddFlag("--option", options...)
	}

	return &Restic{
		logger:       logger,
		resticPath:   cfg.Config.ResticBin,
		ctx:          ctx,
		bucket:       path.Base(cfg.Config.ResticRepository),
		globalFlags:  globalFlags,
		statsHandler: statsHandler,
	}
}
