package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"git.vshn.net/vshn/wrestic/rest"
)

type commandOptions struct {
	print bool
	stdin bool
	rest.Params
}

func initRepository() {
	if _, err := listSnapshots(); err == nil {
		return
	}

	fmt.Println("No repository available, initialising...")
	args := []string{"init"}
	genericCommand(args, commandOptions{print: true})
}

func listSnapshots() ([]snapshot, error) {
	args := []string{"snapshots", "--json", "-q"}
	var timeout int
	var converr error

	if timeout, converr = strconv.Atoi(os.Getenv(listTimeoutEnv)); converr != nil {
		timeout = 30
	}

	done := make(chan bool)
	stdout := make([]string, 0)
	stderr := make([]string, 0)
	go func() {
		stdout, stderr = genericCommand(args, commandOptions{print: false})
		done <- true
	}()
	fmt.Printf("Listing snapshots, timeout: %v\n", timeout)
	select {
	case <-done:
		if len(stderr) > 0 && strings.Contains(stderr[1], "following location?") {
			commandError = nil
			return nil, errors.New("Not initialised yet")
		}
		snapList := make([]snapshot, 0)
		output := strings.Join(stdout, "\n")
		err := json.Unmarshal([]byte(output), &snapList)
		if err != nil {
			fmt.Printf("Error listing snapshots\n%v\n%v", err, strings.Join(stderr, "\n"))
			return nil, err
		}
		availableSnapshots := len(snapList)
		fmt.Printf("%v command:\n%v Snapshots\n", args[0], availableSnapshots)
		metrics.AvailableSnapshots.Set(float64(availableSnapshots))
		metrics.Update(metrics.AvailableSnapshots)
		return snapList, nil
	case <-time.After(time.Duration(timeout) * time.Second):
		commandError = errors.New("connection timed out")
		return nil, commandError
	}
}

func backup() {
	fmt.Println("backing up...")
	args := []string{"backup", backupDir, "--hostname", os.Getenv(hostname)}
	stdout, stderr := genericCommand(args, commandOptions{print: true})
	if commandError == nil {
		parseBackupOutput(stdout, stderr)
	}
}
func forget() {
	// TODO: check for integers
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
	genericCommand(args, commandOptions{print: true})
}

func genericCommand(args []string, options commandOptions) ([]string, []string) {

	// Turn into noop if previous commands failed
	if commandError != nil {
		fmt.Println("Errors occured during previous commands skipping...")
		return nil, nil
	}

	cmd := exec.Command(restic, args...)
	cmd.Env = os.Environ()

	if options.stdin {
		stdout, err := rest.PodExec(options.Params)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			fmt.Println(err)
			commandError = err
			return nil, nil
		}
		if stdout == nil {
			fmt.Println("stdout is nil")
		}
		// This needs to run in a separate thread because
		// cmd.CombinedOutput blocks until the command is finished
		// TODO: this is the place where we could implement some sort of
		// progress bars by wrapping stdin/stdout in a custom reader/writer
		go func() {
			defer stdin.Close()
			_, err := io.Copy(stdin, stdout)
			if err != nil {
				fmt.Println(err)
				commandError = err
				return
			}
		}()
	}

	commandStdout, err := cmd.StdoutPipe()
	commandStderr, err := cmd.StderrPipe()

	finished := make(chan bool, 0)

	stdOutput := make([]string, 0)
	stderrOutput := make([]string, 0)

	cmd.Start()

	go func() {
		stdOutput = collectOutput(commandStdout, options.print)
		finished <- true
	}()

	go func() {
		stderrOutput = collectOutput(commandStderr, options.print)
		finished <- true
	}()

	err = cmd.Wait()
	<-finished
	<-finished

	// Avoid overwriting any errors produced by the
	// copy command
	if commandError == nil {
		commandError = err
	}

	return stdOutput, stderrOutput
}

func collectOutput(output io.ReadCloser, print bool) []string {
	collectedOutput := make([]string, 0)
	scanner := bufio.NewScanner(output)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		m := scanner.Text()
		if print {
			fmt.Println(m)
		}
		collectedOutput = append(collectedOutput, m)
	}
	return collectedOutput
}

func checkCommand() {
	args := []string{"check"}
	parseCheckOutput(genericCommand(args, commandOptions{print: true}))
}

func stdinBackup(backupCommand, pod, container, namespace string) {
	fmt.Printf("backing up via %v stdin...\n", container)
	args := []string{"backup", "--hostname", os.Getenv(hostname) + "-" + container, "--stdin"}
	stdout, stderr := genericCommand(args, commandOptions{
		print: true,
		Params: rest.Params{
			Pod:           pod,
			Container:     container,
			Namespace:     namespace,
			BackupCommand: backupCommand,
		},
		stdin: true,
	})
	parseBackupOutput(stdout, stderr)
}
