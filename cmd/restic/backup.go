package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"git.vshn.net/vshn/wrestic/kubernetes"
)

var folderList []string

func backup() {
	fmt.Println("backing up...")
	files, err := ioutil.ReadDir(getBackupDir())
	if err != nil {
		commandError = err
	}
	folderList = make([]string, 0)
	for _, folder := range files {
		if folder.IsDir() && commandError == nil {
			fmt.Printf("Starting backup for folder %v\n", folder.Name())
			folderList = append(folderList, folder.Name())
			backupFolder(path.Join(getBackupDir(), folder.Name()), folder.Name())
		}
	}
}

func backupFolder(folder, folderName string) {
	args := []string{"backup", folder, "--hostname", os.Getenv(hostname)}
	stdout, stderr := genericCommand(args, commandOptions{print: true})
	parseBackupOutput(stdout, stderr, folderName)
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
	parseBackupOutput(stdout, stderr, "stdin")
}

func parseBackupOutput(stdout, stderr []string, folderName string) {
	if len(stdout) < 6 {
		return
	}
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
		MountedPVCs:     folderList,
	}

	if errorCount > 0 {
		fmt.Println("Following errors occurred during the backup:")
		fmt.Println(strings.Join(stderr, "\n"))
	}

	updateProm(newMetrics, folderName, os.Getenv(hostname))
	postToURL(prepareBackupMetricJSON(newMetrics))

}
