package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"git.vshn.net/vshn/wrestic/s3"
	gzip "github.com/klauspost/pgzip"
)

type restoreStats struct {
	RestoreLocation string   `json:"restore_location,omitempty"`
	SnapshotID      string   `json:"snapshot_ID,omitempty"`
	RestoredFiles   []string `json:"restored_files,omitempty"`
}

func restoreJob(snapshotID, method string) {
	fmt.Println("Starting restore...")

	snapshot := snapshot{}

	snapshots, err := listSnapshots()
	if err != nil {
		fmt.Println(err)
		return
	}

	if snapshotID == "" {
		fmt.Println("No snapshot defined, using latest one.")
		snapshot = snapshots[len(snapshots)-1]
		fmt.Printf("Snapshot %v is being restored.\n", snapshot.Time)
	} else {
		for i := range snapshots {
			if snapshots[i].ID == snapshotID {
				snapshot = snapshots[i]
			}
		}
		if snapshot.ID == "" {
			message := fmt.Sprintf("No Snapshot found with ID %v", snapshotID)
			fmt.Println(message)
			commandError = fmt.Errorf(message)
			return
		}
	}

	method = strings.ToLower(method)

	// TODO: implement some enum here: https://blog.learngoprogramming.com/golang-const-type-enums-iota-bc4befd096d3
	if method == "folder" {
		folderRestore(snapshot)
		return
	}

	if method == "s3" {
		s3Restore(snapshot)
		return
	}

	commandError = fmt.Errorf("%v is not a valid restore type", *restoreType)

}

func folderRestore(snapshot snapshot) {
	restoreDir := getRestoreDir()

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
	if err = postToURL(stats); err != nil {
		commandError = err
	}
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
			var stdErr bytes.Buffer
			cmd.Stderr = &stdErr

			err = cmd.Start()
			if err != nil {
				fmt.Println(err)
				return
			}

			closerChan <- stdOut
			<-closerChan

			err = cmd.Wait()
			if err != nil {
				fmt.Printf("Command failed with: '%v'\n", err)
				fmt.Printf("Output: %v\n", stdErr.String())
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
		defer func() {
			trWriter.Close()
			gzpWriter.Close()
			writePipe.Close()
		}()
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
				fmt.Printf("\n%v\n", err)
				commandError = err
				return
			}
			go file.runFunc()
			reader := <-file.closer
			buffer := bufio.NewReader(reader)
			_, err = io.Copy(trWriter, buffer)
			if err != nil {
				fmt.Printf("\n%v\n", err)
				commandError = err
				return
			}
			file.closer <- reader
			fmt.Println(" done!")
		}
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
