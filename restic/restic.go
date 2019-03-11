package restic

import (
	"encoding/json"
	"os"
	"time"
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
	keepTagEnv        = "KEEP_TAG"
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
	//Restore
	RestoreS3EndpointEnv     = "RESTORE_S3ENDPOINT"
	RestoreS3AccessKeyIDEnv  = "RESTORE_ACCESSKEYID"
	RestoreS3SecretAccessKey = "RESTORE_SECRETACCESSKEY"
	RestoreDirEnv            = "RESTORE_DIR"
)

// snapshot models a restic a single snapshot from the
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

// dummy type to make snapshots sortable by date and satisfy the output.JsonMarshaller interface
type snapList []Snapshot

func (s snapList) Len() int {
	return len(s)
}
func (s snapList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s snapList) Less(i, j int) bool {
	return s[i].Time.Before(s[j].Time)
}

func (s snapList) ToJson() []byte {
	data, _ := json.Marshal(s)
	return data
}

// Restic is an API representation for restic. You can trigger the defined
// command very easily. Every command stores its own output and errors (if any)
// which should provice an easy way to handle logging, outputparsing and error
// handling.
type Restic struct {
	*UnlockStruct
	*RestoreStruct
	*PruneStruct
	*BackupStruct
	*CheckStruct
	*Initrepo
	*ListSnapshotsStruct
}

// New returns a new restic object.
func New(backupDir string) *Restic {
	return &Restic{
		UnlockStruct:        newUnlock(),
		RestoreStruct:       newRestore(),
		PruneStruct:         newPrune(),
		BackupStruct:        newBackup(backupDir, newListSnapshots()),
		CheckStruct:         newCheck(),
		Initrepo:            newInitrepo(),
		ListSnapshotsStruct: newListSnapshots(),
	}
}

// Stats is an interface that returns an interface containing the stats that
// should get pushed via webhook/prom.
type Stats interface {
	GetJson() []byte
	GetProm()
}

func getResticBin() string {
	resticBin := os.Getenv(resticLocationEnv)
	if resticBin == "" {
		resticBin = "restic"
	}
	return resticBin
}
