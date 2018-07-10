package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
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

var (
	check = flag.Bool("check", false, "Set if the container should run a check")

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

func initRepository() {
	if _, err := listSnapshots(); err == nil {
		return
	}

	fmt.Println("No repository available, initialising...")
	args := []string{"init"}
	genericCommand(args, true)
}

func listSnapshots() ([]snapshot, error) {
	args := []string{"snapshots", "--json", "-q"}
	var output []byte
	var timeout int
	var converr error

	if timeout, converr = strconv.Atoi(os.Getenv(listTimeoutEnv)); converr != nil {
		timeout = 30
	}

	done := make(chan []byte)
	go func() { done <- genericCommand(args, false) }()
	fmt.Printf("Listing snapshots, timeout: %v\n", timeout)
	select {
	case output = <-done:
		if strings.Contains(string(output), "following location?") {
			commandError = nil
			return nil, errors.New("Not initialised yet")
		}
		snapList := make([]snapshot, 0)
		err := json.Unmarshal(output, &snapList)
		if err != nil {
			fmt.Printf("Error listing snapshots\n%v\n%v", err, string(output))
			return nil, err
		}
		availableSnapshots := len(snapList)
		fmt.Printf("%v command:\n%v Snapshots\n", args[0], availableSnapshots)
		metrics.AvailableSnapshots.Set(float64(availableSnapshots))
		metrics.Trigger <- metrics.AvailableSnapshots
		return snapList, nil
	case <-time.After(time.Duration(timeout) * time.Second):
		commandError = errors.New("connection timed out")
		return nil, commandError
	}
}

func backup() {
	fmt.Println("backing up...")
	args := []string{"backup", backupDir, "--hostname", os.Getenv(hostname)}
	output := genericCommand(args, true)
	if commandError == nil {
		parseBackupOutput(output)
	}
}
func forget() {
	args := []string{"forget", "--prune"}

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

	fmt.Println("forget params: ", strings.Join(args, " "))
	genericCommand(args, true)
}

func genericCommand(args []string, print bool) []byte {

	// Turn into noop if previous commands failed
	if commandError != nil {
		return nil
	}

	cmd := exec.Command(restic, args...)
	cmd.Env = os.Environ()
	output, exitCode := cmd.CombinedOutput()

	commandError = exitCode

	if print {
		fmt.Printf("%v output:\n%v\n", args[0], string(output))
	}

	return output
}

func checkCommand() {
	args := []string{"check"}
	parseCheckOutput(genericCommand(args, true))
}

func main() {
	//TODO: locking management if f.e. a backup gets interrupted and the lock not
	//cleaned

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
		backup()
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

func parseBackupOutput(output []byte) {
	lines := outputToSlice(output)
	files := strings.Fields(strings.Split(lines[len(lines)-7], ":")[1])
	dirs := strings.Fields(strings.Split(lines[len(lines)-6], ":")[1])

	var errorCount = 0

	for i := range lines {
		if strings.Contains(lines[i], "error") || strings.Contains(lines[i], "Fatal") {
			errorCount++
		}
	}

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
}

func setBackupDir() string {
	if value, ok := os.LookupEnv(backupDirEnv); ok {
		return value
	}
	return "/data"
}

func parseCheckOutput(output []byte) {
	lines := outputToSlice(output)
	lastLine := lines[len(lines)-2]

	if strings.Contains(lastLine, "Fatal") {
		metrics.Errors.Set(1)
		metrics.Trigger <- metrics.Errors
		commandError = errors.New("There was a backup error")
	}
}

func outputToSlice(output []byte) []string {
	stringOutput := string(output)
	return strings.Split(stringOutput, "\n")
}
