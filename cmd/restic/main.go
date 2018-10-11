package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
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
	archive       = flag.Bool("archive", false, "")
	stdinOpts     arrayOpts

	commandError error
	metrics      *resticMetrics
	backupDir    string
)

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
		unlock()
		os.Exit(exit)
	}()

	backupDir = setBackupDir()

	// TODO: simplify this
	if !*restore && !*archive {
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
	} else if *archive {
		archiveJob()
	} else {
		restoreJob(*restoreSnap, *restoreType)
	}

}

func startMetrics() {
	metrics = newResticMetrics(os.Getenv(promURLEnv))
	go metrics.startUpdating()

	metrics.BackupStartTimestamp.SetToCurrentTime()
	metrics.Update(metrics.BackupStartTimestamp)
}

func setBackupDir() string {
	if value, ok := os.LookupEnv(backupDirEnv); ok {
		return value
	}
	return "/data"
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

func prepareBackupMetricJSON(newMetrics rawMetrics) stats {
	snapshotList, err := listSnapshots()
	if err != nil {
		commandError = err
	}

	currentStats := stats{
		Name:          os.Getenv(hostname),
		BackupMetrics: newMetrics,
		Snapshots:     snapshotList,
	}
	return currentStats
}

// postToURL will convert the object you passed to json
// and post it to the defined stats URL
func postToURL(data interface{}) {
	url := os.Getenv(statsURLEnv)
	if url == "" {
		return
	}

	JSONStats, err := json.Marshal(data)
	if err != nil {
		commandError = err
		return
	}

	postBody := bytes.NewReader(JSONStats)

	resp, err := http.Post(url, "application/json", postBody)
	if err != nil || !strings.HasPrefix(resp.Status, "200") {
		httpCode := ""
		if resp == nil {
			httpCode = "http status unavailable"
		} else {
			httpCode = resp.Status
		}
		commandError = fmt.Errorf("Could not send webhook: %v http status code: %v", err, httpCode)
	} else {
		fmt.Printf("Pushed stats to %v\n", url)
	}
}

func setRestoreDir() string {
	if value, ok := os.LookupEnv(restoreDirEnv); ok {
		return value
	}
	return "/restore"
}
