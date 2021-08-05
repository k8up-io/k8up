package cli

import (
	"context"
	"os"
	"path"
	"strings"

	"github.com/go-logr/logr"
)

const (
	// prometheus
	Namespace = "baas"
	Subsystem = "backup_restic"

	// general
	Hostname = "HOSTNAME"

	// Env variable names
	keepLastEnv       = "KEEP_LAST"
	keepHourlyEnv     = "KEEP_HOURLY"
	keepDailyEnv      = "KEEP_DAILY"
	keepWeeklyEnv     = "KEEP_WEEKLY"
	keepMonthlyEnv    = "KEEP_MONTHLY"
	keepYearlyEnv     = "KEEP_YEARLY"
	keepTagEnv        = "KEEP_TAG"
	BackupDirEnv      = "BACKUP_DIR"
	resticLocationEnv = "RESTIC_BINARY"
	resticRepository  = "RESTIC_REPOSITORY"
	resticOptions     = "RESTIC_OPTIONS"

	// Flags for restic
	keepLastArg    = "--keep-last"
	keepHourlyArg  = "--keep-hourly"
	keepDailyArg   = "--keep-daily"
	keepWeeklyArg  = "--keep-weekly"
	keepMonthlyArg = "--keep-monthly"
	keepYearlyArg  = "--keep-yearly"
	keepTagsArg    = "--keep-tag"

	// Restore
	RestoreS3EndpointEnv     = "RESTORE_S3ENDPOINT"
	RestoreS3AccessKeyIDEnv  = "RESTORE_ACCESSKEYID"
	RestoreS3SecretAccessKey = "RESTORE_SECRETACCESSKEY"
	RestoreDirEnv            = "RESTORE_DIR"
)

type ArrayOpts []string

func (a *ArrayOpts) String() string {
	return strings.Join(*a, ", ")
}

func (a *ArrayOpts) BuildArgs() []string {
	argList := []string{}
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
	bin, found := os.LookupEnv(resticLocationEnv)
	if !found {
		bin = "/usr/local/bin/restic"
	}

	repository, found := os.LookupEnv(resticRepository)
	if !found {
		logger.Info(resticRepository + " is undefined")
	}

	globalFlags := Flags{}

	optionString, found := os.LookupEnv(resticOptions)
	options := strings.Split(optionString, ",")
	if found {
		logger.Info("using the following restic options", "options", options)
		globalFlags.AddFlag("--option", options...)
	}

	return &Restic{
		logger:       logger,
		resticPath:   bin,
		ctx:          ctx,
		bucket:       path.Base(repository),
		globalFlags:  globalFlags,
		statsHandler: statsHandler,
	}
}
