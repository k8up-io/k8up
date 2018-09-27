package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	restic   = "/usr/local/bin/restic"
	hostname = "HOSTNAME"
	//Env variable names
	keepLastEnv              = "KEEP_LAST"
	keepHourlyEnv            = "KEEP_HOURLY"
	keepDailyEnv             = "KEEP_DAILY"
	keepWeeklyEnv            = "KEEP_WEEKLY"
	keepMonthlyEnv           = "KEEP_MONTHLY"
	keepYearlyEnv            = "KEEP_YEARLY"
	keepTagEnv               = "KEEP_TAG"
	promURLEnv               = "PROM_URL"
	statsURLEnv              = "STATS_URL"
	backupDirEnv             = "BACKUP_DIR"
	restoreDirEnv            = "RESTORE_DIR"
	listTimeoutEnv           = "BACKUP_LIST_TIMEOUT"
	restoreS3EndpointEnv     = "RESTORE_S3ENDPOINT"
	restoreS3AccessKeyIDEnv  = "RESTORE_ACCESSKEYID"
	restoreS3SecretAccessKey = "RESTORE_SECRETACCESSKEY"
	//Arguments for restic
	keepLastArg    = "--keep-last"
	keepHourlyArg  = "--keep-hourly"
	keepDailyArg   = "--keep-daily"
	keepWeeklyArg  = "--keep-weekly"
	keepMonthlyArg = "--keep-monthly"
	keepYearlyArg  = "--keep-yearly"
)

type arrayOpts []string

func (a *arrayOpts) String() string {
	return "my string representation"
}

func (a *arrayOpts) Set(value string) error {
	*a = append(*a, value)
	return nil
}

var (
	check         = flag.Bool("check", false, "Set if the container should run a check")
	stdin         = flag.Bool("stdin", false, "Set to enable stdin backup")
	prune         = flag.Bool("prune", false, "Set if the container should run a prune")
	restore       = flag.Bool("restore", false, "Wheter or not a restore should be done")
	restoreSnap   = flag.String("restoreSnap", "", "Snapshot ID, if empty takes the latest snapshot")
	verifyRestore = flag.Bool("verifyRestore", false, "If the restore should get verified, only for PVCs restore")
	restoreType   = flag.String("restoreType", "", "Type of this restore, folder or S3")
	restoreFilter = flag.String("restoreFilter", "", "Simple filter to define what should get restored. For example the PVC name")
	stdinOpts     arrayOpts

	commandError error
	metrics      *resticMetrics
	backupDir    string
)

// snapshot models a restic a single snapshot from the
// snapshots --json subcommand.
type snapshot struct {
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

type stats struct {
	Name          string     `json:"name"`
	BackupMetrics rawMetrics `json:"backup_metrics"`
	Snapshots     []snapshot `json:"snapshots"`
}

func main() {
	//TODO: locking management if f.e. a backup gets interrupted and the lock not
	//cleaned

	flag.Var(&stdinOpts, "arrayOpts", "Options needed for the stding backup. Format: command,pod,container")

	exit := 0

	flag.Parse()

	startMetrics()

	defer func() {
		if commandError != nil {
			fmt.Println("Error occurred: ", commandError)
			exit = 1
		}
		metrics.BackupEndTimestamp.SetToCurrentTime()
		metrics.Update(metrics.BackupEndTimestamp)
		os.Exit(exit)
	}()

	backupDir = setBackupDir()

	if !*restore {
		initRepository()

		if *check {
			checkCommand()
		} else {
			if !*stdin && !*prune {
				backup()
			} else if !*prune {
				fmt.Println("Backup commands detected")
				for _, stdin := range stdinOpts {
					optsSplitted := strings.Split(stdin, ",")
					if len(optsSplitted) != 4 {
						commandError = fmt.Errorf("not enough arguments %v for stdin", stdin)
					}
					stdinBackup(optsSplitted[0], optsSplitted[1], optsSplitted[2], optsSplitted[3])
				}
				// After doing all backups via stdin don't forget todo the normal one
				if _, err := os.Stat(backupDir); os.IsNotExist(err) {
					fmt.Printf("%v does not exist, skipping file backup\n", backupDir)
				} else {
					backup()
				}
			} else {
				forget()
			}
			listSnapshots()
		}
	} else {
		restoreJob()
	}

}

func startMetrics() {
	metrics = newResticMetrics(os.Getenv(promURLEnv))
	go metrics.startUpdating()

	metrics.BackupStartTimestamp.SetToCurrentTime()
	metrics.Update(metrics.BackupStartTimestamp)
}

func parseBackupOutput(stdout, stderr []string) {
	files := strings.Fields(strings.Split(stdout[len(stdout)-6], ":")[1])
	dirs := strings.Fields(strings.Split(stdout[len(stdout)-5], ":")[1])

	var errorCount = len(stderr)

	newFiles, err := strconv.Atoi(files[0])
	changedFiles, err := strconv.Atoi(files[2])
	unmodifiedFiles, err := strconv.Atoi(files[4])

	newDirs, err := strconv.Atoi(dirs[0])
	changedDirs, err := strconv.Atoi(dirs[2])
	unmodifiedDirs, err := strconv.Atoi(dirs[4])

	if err != nil {
		errorMessage := fmt.Sprintln("There was a problem convertig the metrics: ", err)
		fmt.Println(errorMessage)
		commandError = errors.New(errorMessage)
		return
	}

	if commandError != nil {
		errorCount++
	}

	newMetrics := rawMetrics{
		NewDirs:         float64(newDirs),
		NewFiles:        float64(newFiles),
		ChangedFiles:    float64(changedFiles),
		UnmodifiedFiles: float64(unmodifiedFiles),
		ChangedDirs:     float64(changedDirs),
		UnmodifiedDirs:  float64(unmodifiedDirs),
	}

	updateProm(newMetrics)
	postToURL(newMetrics)

	if errorCount > 0 && commandError == nil {
		commandError = fmt.Errorf("there where %v errors", errorCount)
	}
}

func setBackupDir() string {
	if value, ok := os.LookupEnv(backupDirEnv); ok {
		return value
	}
	return "/data"
}

func parseCheckOutput(stdout, stderr []string) {
	metrics.Errors.Set(float64(len(stderr)))
	metrics.Update(metrics.Errors)
	if len(stderr) > 0 {
		commandError = errors.New("There was at least one backup error")
	}
}

func outputToSlice(output []byte) []string {
	stringOutput := string(output)
	return strings.Split(stringOutput, "\n")
}

func updateProm(newMetrics rawMetrics) {
	metrics.NewFiles.Set(newMetrics.NewFiles)
	metrics.Update(metrics.NewFiles)
	metrics.ChangedFiles.Set(newMetrics.ChangedFiles)
	metrics.Update(metrics.ChangedFiles)
	metrics.UnmodifiedFiles.Set(newMetrics.UnmodifiedFiles)
	metrics.Update(metrics.UnmodifiedFiles)
	metrics.NewDirs.Set(newMetrics.NewDirs)
	metrics.Update(metrics.NewDirs)
	metrics.ChangedDirs.Set(newMetrics.ChangedDirs)
	metrics.Update(metrics.ChangedDirs)
	metrics.UnmodifiedDirs.Set(newMetrics.UnmodifiedDirs)
	metrics.Update(metrics.UnmodifiedDirs)
	metrics.Errors.Set(newMetrics.Errors)
	metrics.Update(metrics.Errors)
}

func postToURL(newMetrics rawMetrics) {
	url := os.Getenv(statsURLEnv)
	if url == "" {
		return
	}

	snapshotList, err := listSnapshots()
	if err != nil {
		commandError = err
	}

	currentStats := stats{
		Name:          os.Getenv(hostname),
		BackupMetrics: newMetrics,
		Snapshots:     snapshotList,
	}

	JSONStats, err := json.Marshal(currentStats)
	if err != nil {
		commandError = err
		return
	}

	postBody := bytes.NewReader(JSONStats)

	http.Post(url, "application/json", postBody)

}

func setRestoreDir() string {
	if value, ok := os.LookupEnv(restoreDirEnv); ok {
		return value
	}
	return "/restore"
}
