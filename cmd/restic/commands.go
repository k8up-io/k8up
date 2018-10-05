package main

import (
	"archive/tar"
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

	gzip "github.com/klauspost/pgzip"

	"git.vshn.net/vshn/wrestic/rest"
	"git.vshn.net/vshn/wrestic/s3"
)

type commandOptions struct {
	print bool
	stdin bool
	rest.Params
}

type restoreStats struct {
	RestoreLocation string   `json:"restore_location,omitempty"`
	SnapshotID      string   `json:"snapshot_ID,omitempty"`
	RestoredFiles   []string `json:"restored_files,omitempty"`
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

	finished := make(chan error, 0)

	stdOutput := make([]string, 0)
	stderrOutput := make([]string, 0)

	cmd.Start()

	go func() {
		var collectErr error
		stdOutput, collectErr = collectOutput(commandStdout, options.print)
		finished <- collectErr
	}()

	go func() {
		var collectErr error
		stderrOutput, collectErr = collectOutput(commandStderr, options.print)
		finished <- collectErr
	}()

	collectErr1 := <-finished
	collectErr2 := <-finished
	err = cmd.Wait()

	// Avoid overwriting any errors produced by the
	// copy command
	if commandError == nil {
		if err != nil {
			commandError = err
		}
		if collectErr1 != nil {
			commandError = collectErr1
		}
		if collectErr2 != nil {
			commandError = collectErr2
		}
	}

	return stdOutput, stderrOutput
}

func collectOutput(output io.ReadCloser, print bool) ([]string, error) {
	collectedOutput := make([]string, 0)
	scanner := bufio.NewScanner(output)
	buff := make([]byte, 64*1024*1024)
	scanner.Buffer(buff, 64*1024*1024)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		m := scanner.Text()
		if print {
			fmt.Println(m)
		}
		collectedOutput = append(collectedOutput, m)
	}
	return collectedOutput, scanner.Err()
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

func restoreJob() {
	fmt.Println("Starting restore...")

	snapshot := snapshot{}

	snapshots, err := listSnapshots()
	if err != nil {
		fmt.Println(err)
		return
	}

	if *restoreSnap == "" {
		fmt.Println("No snapshot defined, using latest one.")
		snapshot = snapshots[len(snapshots)-1]
		fmt.Printf("Snapshot %v is being restored.\n", snapshot.Time)
	} else {
		for i := range snapshots {
			if snapshots[i].ID == *restoreSnap {
				snapshot = snapshots[i]
			}
		}
		if snapshot.ID == "" {
			message := fmt.Sprintf("No Snapshot found with ID %v", *restoreSnap)
			fmt.Println(message)
			commandError = fmt.Errorf(message)
			return
		}
	}

	// TODO: implement some enum here: https://blog.learngoprogramming.com/golang-const-type-enums-iota-bc4befd096d3
	if *restoreType == "folder" {
		folderRestore(snapshot)
		return
	}

	if *restoreType == "s3" {
		s3Restore(snapshot)
		return
	}

	commandError = fmt.Errorf("%v is not a valid restore type", *restoreType)

}

func folderRestore(snapshot snapshot) {
	restoreDir := setRestoreDir()

	args := []string{"restore", snapshot.ID, "--target", restoreDir}

	if *restoreFilter != "" {
		args = append(args, "--include", *restoreFilter)
	}

	if *verifyRestore {
		args = append(args, "--verify")
	}

	_, stdErr := genericCommand(args, commandOptions{print: true})
	notIgnoredErrors := 0
	for _, errLine := range stdErr {
		if !strings.Contains(errLine, "Lchown") {
			notIgnoredErrors++
		}
	}
	if notIgnoredErrors > 0 {
		commandError = fmt.Errorf("There were %v unignored errors, please have a look", notIgnoredErrors)
	}
}

func s3Restore(snapshot snapshot) {
	fmt.Println("S3 chosen as restore destination")
	fileList := listFilesInSnapshot(snapshot)
	readers, err := createFileReaders(snapshot, fileList)
	if err != nil {
		commandError = err
		return
	}

	endpoint := os.Getenv(restoreS3EndpointEnv)
	snapDate := snapshot.Time.Format(time.RFC3339)
	fileName := fmt.Sprintf("backup-%v-%v.tar.gz", snapshot.Hostname, snapDate)
	stats := &restoreStats{
		RestoreLocation: fmt.Sprintf("%v/%v", endpoint, fileName),
		SnapshotID:      snapshot.ID,
	}

	s3Client := s3.New(endpoint, os.Getenv(restoreS3AccessKeyIDEnv), os.Getenv(restoreS3SecretAccessKey))
	err = s3Client.Connect()
	if err != nil {
		commandError = err
		return
	}
	stream := tarGz(readers, stats)
	upload := s3.UploadObject{
		ObjectStream: stream,
		Name:         fileName,
	}
	err = s3Client.Upload(upload)
	if err != nil {
		commandError = err
	}
	postToURL(stats)
}

func listFilesInSnapshot(snapshot snapshot) []string {
	// TODO: if there's a problem with many files this may need
	// rewriting so it uses pipes
	args := []string{"ls", "-l", snapshot.ID}
	fmt.Printf("Listing files in snapshot %v\n", snapshot.Time)
	stdOut, _ := genericCommand(args, commandOptions{print: false})
	return stdOut
}

type restoreFile struct {
	name    string
	mode    int64
	size    int64
	runFunc func()
	uid     int
	gid     int
	closer  chan io.ReadCloser
}

func createFileReaders(snapshot snapshot, fileList []string) ([]restoreFile, error) {
	// This case is so special we can't use genericCommand for this
	args := []string{"dump", snapshot.ID}

	fmt.Println("Adding files to the restore")

	filesToProcess := []restoreFile{}

	for i, file := range fileList {
		//Skip first entry
		if i == 0 {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(file))
		scanner.Split(bufio.ScanWords)
		j := 0
		add := false
		// TODO: Next version of restic will most likely have
		// a json flag for the ls command.
		tmpFile := restoreFile{}
		for scanner.Scan() {
			m := scanner.Text()

			if j == 0 {
				if !strings.HasPrefix(m, "d") {
					add = true
					perms := m[1:len(m)]
					usrPerm := perms[0:3]
					groupPerm := perms[3:6]
					everyOne := perms[6:9]
					tmpPermString := "0" + parsePerm(usrPerm)
					tmpPermString = tmpPermString + parsePerm(groupPerm)
					tmpPermString = tmpPermString + parsePerm(everyOne)
					tmpFile.mode, _ = strconv.ParseInt(tmpPermString, 0, 64)
				} else {
					j++
					continue
				}
			}

			if j == 3 && add {
				size, err := strconv.ParseInt(m, 10, 64)
				tmpFile.size = size
				if err != nil {
					return nil, err
				}
			}

			if j == 1 && add {
				uid, err := strconv.Atoi(m)
				tmpFile.uid = uid
				if err != nil {
					return nil, err
				}
			}

			if j == 2 && add {
				gid, err := strconv.Atoi(m)
				tmpFile.gid = gid
				if err != nil {
					return nil, err
				}
			}

			if j == 6 && add {
				if *restoreFilter != "" {
					if strings.Contains(m, *restoreFilter) {
						tmpFile.name = m
					} else {
						add = false
					}
				} else {
					tmpFile.name = m
				}
			}
			j++
		}
		if add {
			filesToProcess = append(filesToProcess, tmpFile)
		}
	}

	for i := range filesToProcess {
		tmpArgs := append(args, filesToProcess[i].name)
		cmd := exec.Command(restic, tmpArgs...)
		cmd.Env = os.Environ()
		closerChan := make(chan io.ReadCloser, 0)
		filesToProcess[i].closer = closerChan
		filesToProcess[i].runFunc = func() {

			stdOut, err := cmd.StdoutPipe()
			if err != nil {
				return
			}

			err = cmd.Start()
			if err != nil {
				fmt.Println(err)
				return
			}

			closerChan <- stdOut
			<-closerChan

			err = cmd.Wait()
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}

	return filesToProcess, nil
}

func tarGz(files []restoreFile, stats *restoreStats) io.Reader {
	readPipe, writePipe := io.Pipe()
	gzpWriter := gzip.NewWriter(writePipe)
	trWriter := tar.NewWriter(gzpWriter)

	go func() {
		for _, file := range files {
			stats.RestoredFiles = append(stats.RestoredFiles, file.name)
			fmt.Printf("Compressing %v...", file.name)
			header := &tar.Header{
				Name: file.name,
				Mode: file.mode,
				Size: file.size,
				Uid:  file.uid,
				Gid:  file.gid,
			}

			err := trWriter.WriteHeader(header)
			if err != nil {
				fmt.Println(err)
			}
			go file.runFunc()
			reader := <-file.closer
			buffer := bufio.NewReader(reader)
			_, err = io.Copy(trWriter, buffer)
			if err != nil {
				fmt.Println(err)
				commandError = err
				trWriter.Close()
				gzpWriter.Close()
				writePipe.Close()
				return
			}
			file.closer <- reader
			fmt.Println(" done!")
		}
		trWriter.Close()
		gzpWriter.Close()
		writePipe.Close()
	}()

	return readPipe
}

func parsePerm(perm string) string {
	permInt := 0
	for _, char := range perm {
		if char == 'r' {
			permInt = permInt + 4
		}
		if char == 'w' {
			permInt = permInt + 2
		}
		if char == 'x' {
			permInt = permInt + 1
		}
	}
	return strconv.Itoa(permInt)
}

func unlock() {
	fmt.Println("Removing locks...")
	args := []string{"unlock"}
	genericCommand(args, commandOptions{print: true})
}
