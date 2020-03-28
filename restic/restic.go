package restic

import (
	"context"
	"os"
	"path"
	"strings"

	"github.com/go-logr/logr"
)

const (
	//prometheus
	Namespace = "baas"
	Subsystem = "backup_restic"
	//general
	Hostname = "HOSTNAME"
	//Env variable names
	keepLastEnv       = "KEEP_LAST"
	keepHourlyEnv     = "KEEP_HOURLY"
	keepDailyEnv      = "KEEP_DAILY"
	keepWeeklyEnv     = "KEEP_WEEKLY"
	keepMonthlyEnv    = "KEEP_MONTHLY"
	keepYearlyEnv     = "KEEP_YEARLY"
	keepTagsEnv       = "KEEP_TAGS"
	promURLEnv        = "PROM_URL"
	statsURLEnv       = "STATS_URL"
	BackupDirEnv      = "BACKUP_DIR"
	restoreDirEnv     = "RESTORE_DIR"
	listTimeoutEnv    = "BACKUP_LIST_TIMEOUT"
	resticLocationEnv = "RESTIC_BINARY"
	repositoryEnv     = "RESTIC_REPOSITORY"
	//Arguments for restic
	keepLastArg    = "--keep-last"
	keepHourlyArg  = "--keep-hourly"
	keepDailyArg   = "--keep-daily"
	keepWeeklyArg  = "--keep-weekly"
	keepMonthlyArg = "--keep-monthly"
	keepYearlyArg  = "--keep-yearly"
	keepTagsArg    = "--keep-tag"
	//Restore
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
	resticPath   string
	logger       logr.Logger
	snapshots    []Snapshot
	ctx          context.Context
	bucket       string
	statsHandler StatsHandler
}

// New returns a new Restic reference
func New(ctx context.Context, logger logr.Logger, statsHandler StatsHandler) *Restic {

	bin := os.Getenv(resticLocationEnv)
	if bin == "" {
		bin = "/usr/local/bin/restic"
	}

	return &Restic{
		logger:       logger,
		resticPath:   bin,
		ctx:          ctx,
		bucket:       path.Base(os.Getenv("RESTIC_REPOSITORY")),
		statsHandler: statsHandler,
	}
}
