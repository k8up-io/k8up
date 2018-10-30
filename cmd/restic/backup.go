package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"git.vshn.net/vshn/wrestic/kubernetes"
)

func backup() {
	fmt.Println("backing up...")
	args := []string{"backup", backupDir, "--hostname", os.Getenv(hostname)}
	stdout, stderr := genericCommand(args, commandOptions{print: true})
	if commandError == nil {
		parseBackupOutput(stdout, stderr)
	}
}

func stdinBackup(backupCommand, pod, container, namespace string) {
	fmt.Printf("backing up via %v stdin...\n", container)
	args := []string{"backup", "--hostname", os.Getenv(hostname) + "-" + container, "--stdin"}
	stdout, stderr := genericCommand(args, commandOptions{
		print: true,
		Params: kubernetes.Params{
			Pod:           pod,
			Container:     container,
			Namespace:     namespace,
			BackupCommand: backupCommand,
		},
		stdin: true,
	})
	parseBackupOutput(stdout, stderr)
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

	folders, err := ioutil.ReadDir(setBackupDir())
	if err != nil {
		commandError = err
		return
	}
	mountedPVCs := []string{}
	for _, folder := range folders {
		mountedPVCs = append(mountedPVCs, folder.Name())
	}

	newMetrics := rawMetrics{
		NewDirs:         float64(newDirs),
		NewFiles:        float64(newFiles),
		ChangedFiles:    float64(changedFiles),
		UnmodifiedFiles: float64(unmodifiedFiles),
		ChangedDirs:     float64(changedDirs),
		UnmodifiedDirs:  float64(unmodifiedDirs),
		MountedPVCs:     mountedPVCs,
	}

	updateProm(newMetrics)
	postToURL(prepareBackupMetricJSON(newMetrics))

	if errorCount > 0 && commandError == nil {
		commandError = fmt.Errorf("there where %v errors", errorCount)
	}
}
