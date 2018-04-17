package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	//Arguments for restic
	keepLastArg    = "--keep-last"
	keepHourlyArg  = "--keep-hourly"
	keepDailyArg   = "--keep-daily"
	keepWeeklyArg  = "--keep-weekly"
	keepMonthlyArg = "--keep-monthly"
	keepYearlyArg  = "--keep-yearly "
)

var (
	check = flag.Bool("check", false, "Set if the container should run a check")

	commandError error
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
	output := genericCommand(args, false)
	snapList := make([]snapshot, 0)
	err := json.Unmarshal(output, &snapList)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%v command:\n%v Snapshots\n", args[0], len(snapList))
	return snapList, nil
}

func backup() {
	fmt.Println("backing up...")
	args := []string{"backup", "/data", "--hostname", os.Getenv(hostname)}
	genericCommand(args, true)
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
	output, exitCode := cmd.Output()
	if exitCode != nil {
		return output
	}
	if print {
		fmt.Printf("%v output:\n%v\n", args[0], string(output))
	}
	return output
}

func checkCommand() {
	args := []string{"check"}
	genericCommand(args, true)
}

func main() {
	//TODO: locking management if f.e. a backup gets interrupted and the lock not
	//cleaned

	flag.Parse()

	if !*check {
		initRepository()
		backup()
		forget()
	} else {
		checkCommand()
	}

	if commandError != nil {
		fmt.Println("Error occurred: ", commandError)
		os.Exit(1)
	}
}
