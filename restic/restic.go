package restic

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"git.vshn.net/vshn/wrestic/output"
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

// WebhookSender describes an object that receives a JsonMarshaller and
// sends it to its target.
type WebhookSender interface {
	TriggerHook(data output.JsonMarshaller)
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
	commandState *commandState
}

// command contians the currently running genericCommand
type commandState struct {
	running *genericCommand
}

// New returns a new restic object.
func New(backupDir string, webhookSender WebhookSender) *Restic {
	commandState := &commandState{}
	snapshotLister := newListSnapshots(commandState)
	return &Restic{
		UnlockStruct:        newUnlock(commandState),
		RestoreStruct:       newRestore(commandState),
		PruneStruct:         newPrune(snapshotLister, webhookSender, commandState),
		BackupStruct:        newBackup(backupDir, snapshotLister, webhookSender, commandState),
		CheckStruct:         newCheck(commandState),
		Initrepo:            newInitrepo(commandState),
		ListSnapshotsStruct: snapshotLister,
		commandState:        commandState,
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

func getBucket() string {
	bucket := ""
	repo := strings.Replace(os.Getenv(repositoryEnv), "s3:", "", 1)

	u, err := url.Parse(repo)
	if err == nil {
		bucket = strings.Replace(u.Path, "/", "", 1)
	}

	return bucket
}

// SendSignal will send a signal to the currently running restic process
// so it can shutdown cleanly.
func (r *Restic) SendSignal(sig os.Signal) {
	if r.commandState.running != nil {
		err := r.commandState.running.sendSignal(sig)
		if err != nil {
			fmt.Printf("error sending signal to restic: %s\n", err)
		}
	}
}
