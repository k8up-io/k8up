package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	restic   = "/usr/local/bin/restic"
	hostname = "HOSTNAME"
	//Env variable names
	keepLastEnv    = "KEEP_LAST"
	keepHourlyEnv  = "KEEP_HOURLY"
	keepDailyEnv   = "KEEP_DAILY"
	keepWeeklyEnv  = "KEEP_WEEKLY"
	keepMonthlyEnv = "KEEP_MONTHLY"
	keepYearlyEnv  = "KEEP_YEARLY"
	keepTagEnv     = "KEEP_TAG"
	promURLEnv     = "PROM_URL"
	backupDirEnv   = "BACKUP_DIR"
	listTimeoutEnv = "BACKUP_LIST_TIMEOUT"
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
	check     = flag.Bool("check", false, "Set if the container should run a check")
	stdin     = flag.Bool("stdin", false, "Set to enable stdin backup")
	stdinOpts arrayOpts

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
		metrics.Trigger <- metrics.BackupEndTimestamp
		// Block a second to transmit the metrics
		time.Sleep(1 * time.Second)
		os.Exit(exit)
	}()

	backupDir = setBackupDir()

	if !*check {
		initRepository()
		if !*stdin {
			backup()
		} else {
			for _, stdin := range stdinOpts {
				optsSplitted := strings.Split(stdin, ",")
				if len(optsSplitted) != 4 {
					commandError = fmt.Errorf("not enough arguments %v for stdin", stdin)
				}
				stdinBackup(optsSplitted[0], optsSplitted[1], optsSplitted[2], optsSplitted[3])
			}
			// After doing all backups via stdin don't forget todo the normal one
			backup()
		}
		forget()
	} else {
		checkCommand()
	}

}

func startMetrics() {
	metrics = newResticMetrics(os.Getenv(promURLEnv))
	go metrics.startUpdating()

	metrics.BackupStartTimestamp.SetToCurrentTime()
	metrics.Trigger <- metrics.BackupStartTimestamp
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

	metrics.NewFiles.Set(float64(newFiles))
	metrics.Trigger <- metrics.NewFiles
	metrics.ChangedFiles.Set(float64(changedFiles))
	metrics.Trigger <- metrics.ChangedFiles
	metrics.UnmodifiedFiles.Set(float64(unmodifiedFiles))
	metrics.Trigger <- metrics.UnmodifiedFiles
	metrics.NewDirs.Set(float64(newDirs))
	metrics.Trigger <- metrics.NewDirs
	metrics.ChangedDirs.Set(float64(changedDirs))
	metrics.Trigger <- metrics.ChangedDirs
	metrics.UnmodifiedDirs.Set(float64(unmodifiedDirs))
	metrics.Trigger <- metrics.UnmodifiedDirs
	metrics.Errors.Set(float64(errorCount))
	metrics.Trigger <- metrics.Errors

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
	metrics.Trigger <- metrics.Errors
	if len(stderr) > 0 {
		commandError = errors.New("There was at least one backup error")
	}
}

func outputToSlice(output []byte) []string {
	stringOutput := string(output)
	return strings.Split(stringOutput, "\n")
}
